package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ------------------------------
// Глобальные переменные (как в bash)
// ------------------------------
const (
	WATCH_DIR         = "/var/www"
	NGINX_AVAILABLE   = "/etc/nginx/sites-available"
	NGINX_ENABLED     = "/etc/nginx/sites-enabled"
	TPL_NOSSL_NOWWW   = "/root/auto_deploy/templates/nossl_nowww.conf.j2"
	TPL_NOSSL_WWW     = "/root/auto_deploy/templates/nossl_www.conf.j2"
	TPL_SSL_NOWWW     = "/root/auto_deploy/templates/ssl_nowww.conf.j2"
	TPL_SSL_WWW       = "/root/auto_deploy/templates/ssl_www.conf.j2"
	WP_LOG            = "/root/auto_deploy/deploy_wp.txt"
	LOG_DIR           = "/root/auto_deploy/log"
	CLOUDFLARE_TXT    = "/root/auto_deploy/cloudflare.txt"
	SERVER_IP_COMMAND = `hostname -I | awk '{print $1}'`
)

// ------------------------------
// Глобальные (вычислим при старте)
// ------------------------------
var (
	SERVER_IP     string
	TODAY         string
	LOG_FILE      string
	useCloudflare = true // Флаг для использования CloudFlare
)

// ------------------------------
// Вспомогательные функции
// ------------------------------

// runCmd запускает системную команду (name + args) и возвращает ошибку (если есть).
// stdout/stderr транслируется в общий logger (через io.MultiWriter).
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runCmdOutput запускает команду и возвращает её stdout как строку (trimmed).
// При ошибке возвращает err.
func runCmdOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// sleepSec — пауза в секундах с логированием
func sleepSec(sec int) {
	log.Printf("[INFO] Спим %d сек...", sec)
	time.Sleep(time.Duration(sec) * time.Second)
}

// ------------------------------
// (1) Очистка логов старше 7 дней
// ------------------------------
func cleanOldLogs() {
	log.Println("[INFO] Очистка логов старше 7 дней...")
	filepath.Walk(LOG_DIR, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if time.Since(info.ModTime()).Hours() > 24*7 {
			os.Remove(path)
		}
		return nil
	})
}

// ------------------------------
// (2) Проверка cloudflare.txt (не пуст?)
// ------------------------------
func checkCloudflareFileSimple() bool {
	fi, err := os.Stat(CLOUDFLARE_TXT)
	if err != nil {
		log.Println("[ERROR]", CLOUDFLARE_TXT, "не существует! (Ошибка 550)")
		return false
	}
	if fi.Size() == 0 {
		log.Println("[ERROR]", CLOUDFLARE_TXT, "пуст! (Ошибка 550)")
		return false
	}
	log.Println("[INFO] cloudflare.txt не пуст, продолжаем...")
	return true
}

// ------------------------------
// (3) Получить суффикс ошибки
// ------------------------------
func getErrorSuffix(idx string, etype string) string {
	if etype == "cloudflare" {
		return "550"
	}
	if etype == "check_text" {
		return "551"
	}
	switch idx {
	case "0":
		return "000"
	case "1":
		return "001"
	case "2":
		return "002"
	case "3":
		return "003"
	case "4":
		return "004"
	case "5":
		return "005"
	case "6":
		return "006"
	case "7":
		return "007"
	default:
		return "550"
	}
}

// ------------------------------
// (4) Проверка домена в Cloudflare (3 попытки, 15 сек)
//     Возвращает true/false
// ------------------------------
var CLOUDFLARE_ZONE_ID, CLOUDFLARE_EMAIL, CLOUDFLARE_API_KEY string

func checkDomainCloudflare(domain string) bool {
	log.Println("[INFO] Проверяем домен", domain, "в Cloudflare...")
	data, err := os.ReadFile(CLOUDFLARE_TXT)
	if err != nil {
		log.Printf("[ERROR] Невозможно прочитать %s: %v", CLOUDFLARE_TXT, err)
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}
		email := parts[0]
		token := parts[1]
		zoneResp, err := runCmdOutput("curl", "-s", "-X", "GET",
			fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", domain),
			"-H", fmt.Sprintf("X-Auth-Email: %s", email),
			"-H", fmt.Sprintf("X-Auth-Key: %s", token),
			"-H", "Content-Type: application/json",
		)
		if err != nil {
			log.Println("[WARN] Ошибка curl:", err)
			continue
		}
		zoneID := parseJSON(zoneResp, ".result[0].id")
		zoneName := parseJSON(zoneResp, ".result[0].name")
		zoneStatus := parseJSON(zoneResp, ".result[0].status")
		if zoneID != "null" && zoneName == domain {
			log.Printf("[INFO] Найден ZONE_ID=%s, статус=%s", zoneID, zoneStatus)
			attempts := 0
			for zoneStatus != "active" && attempts < 3 {
				log.Println("[INFO] Ждём 15 сек, чтобы зона стала active...")
				time.Sleep(15 * time.Second)
				zoneResp2, _ := runCmdOutput("curl", "-s", "-X", "GET",
					fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", domain),
					"-H", fmt.Sprintf("X-Auth-Email: %s", email),
					"-H", fmt.Sprintf("X-Auth-Key: %s", token),
					"-H", "Content-Type: application/json",
				)
				zoneStatus = parseJSON(zoneResp2, ".result[0].status")
				attempts++
			}
			if zoneStatus == "active" {
				dnsResp, err := runCmdOutput("curl", "-s", "-X", "GET",
					fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s", zoneID, domain),
					"-H", fmt.Sprintf("X-Auth-Email: %s", email),
					"-H", fmt.Sprintf("X-Auth-Key: %s", token),
					"-H", "Content-Type: application/json",
				)
				if err != nil {
					log.Println("[WARN] Ошибка curl DNS:", err)
					continue
				}
				dnsContent := parseJSON(dnsResp, ".result[0].content")
				dnsName := parseJSON(dnsResp, ".result[0].name")
				if dnsName == domain && dnsContent == SERVER_IP {
					log.Printf("[INFO] DNS=%s совпадает с %s", dnsContent, SERVER_IP)
					CLOUDFLARE_ZONE_ID = zoneID
					CLOUDFLARE_EMAIL = email
					CLOUDFLARE_API_KEY = token
					return true
				}
			}
		}
	}
	log.Printf("[ERROR] Не нашли активную зону для %s c IP=%s", domain, SERVER_IP)
	return false
}

// parseJSON — простая функция для извлечения поля через jq (без дополнительного парсинга)
func parseJSON(jsonText, jqFilter string) string {
	out, err := runCmdOutput("jq", "-r", jqFilter)
	if err == nil && out != "" {
		return out
	}
	cmd := exec.Command("jq", "-r", jqFilter)
	cmd.Stdin = strings.NewReader(jsonText)
	raw, err2 := cmd.Output()
	if err2 == nil {
		return strings.TrimSpace(string(raw))
	}
	return ""
}

// ------------------------------
// (5) set_cf_ssl_mode (flexible|full), sleep 5
// ------------------------------
func setCFSSLMode(mode string) {
	z := CLOUDFLARE_ZONE_ID
	email := CLOUDFLARE_EMAIL
	token := CLOUDFLARE_API_KEY
	log.Printf("[INFO] Ставим SSL=%s (zone=%s)...", mode, z)
	resp, err := runCmdOutput("curl", "-s", "-X", "PATCH",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/settings/ssl", z),
		"-H", "Content-Type: application/json",
		"-H", fmt.Sprintf("X-Auth-Email: %s", email),
		"-H", fmt.Sprintf("X-Auth-Key: %s", token),
		"-d", fmt.Sprintf(`{"id":"ssl","value":"%s"}`, mode),
	)
	if err == nil {
		success := parseJSON(resp, ".success")
		if success == "true" {
			log.Printf("[INFO] ssl=%s -> success", mode)
		} else {
			log.Printf("[WARN] ssl=%s -> not successful: %s", mode, resp)
		}
	} else {
		log.Printf("[WARN] set_cf_ssl_mode(%s) ошибка: %v", mode, err)
	}
	sleepSec(0)
}

// ------------------------------
// (6) apply_default_cf_settings
// ------------------------------
func applyDefaultCFSettings() {
	z := CLOUDFLARE_ZONE_ID
	email := CLOUDFLARE_EMAIL
	token := CLOUDFLARE_API_KEY
	log.Println("[INFO] Применяем дефолтные настройки CF в новом порядке...")
	patchSetting := func(key, val string) {
		resp, err := runCmdOutput("curl", "-s", "-X", "PATCH",
			fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/settings/%s", z, key),
			"-H", "Content-Type: application/json",
			"-H", fmt.Sprintf("X-Auth-Email: %s", email),
			"-H", fmt.Sprintf("X-Auth-Key: %s", token),
			"-d", fmt.Sprintf(`{"id":"%s","value":"%s"}`, key, val),
		)
		if err == nil {
			success := parseJSON(resp, ".success")
			if success == "true" {
				log.Printf("[INFO] %s=%s -> success", key, val)
			} else {
				log.Printf("[WARN] %s=%s -> not successful: %s", key, val, resp)
			}
		} else {
			log.Printf("[WARN] patchSetting(%s=%s) ошибка: %v", key, val, err)
		}
		sleepSec(0)
	}
	patchSetting("tls_1_3", "off")
	patchSetting("always_use_https", "off")
	patchSetting("0rtt", "on")
	patchSetting("automatic_https_rewrites", "on")
	patchSetting("brotli", "on")
	patchSetting("http3", "on")
	patchSetting("opportunistic_encryption", "on")
	patchSetting("security_level", "essentially_off")
	patchSetting("speed_brain", "on")
}

// ------------------------------
// (7) Генерировать 9 символов
// ------------------------------
func generate9chars() (string, error) {
	return runCmdOutput("bash", "-c", "openssl rand -base64 12 | tr -dc A-Za-z0-9 | head -c9")
}

// ------------------------------
// (8) Три попытки проверить текст
// ------------------------------
func checkText3Attempts(domain, txt string) bool {
	attempt := 0
	for attempt < 3 {
		log.Printf("[INFO] Проверяем curl https://%s (попытка %d)...", domain, attempt+1)
		checkOutput, err := runCmdOutput("curl", "-k", "-s", fmt.Sprintf("https://%s", domain))
		if err == nil && strings.Contains(checkOutput, txt) {
			log.Printf("[INFO] Текст %s найден (попытка %d)!", txt, attempt+1)
			sleepSec(3)
			return true
		}
		log.Printf("[WARN] Не нашли текст %s, ждём 5 сек...", txt)
		time.Sleep(5 * time.Second)
		attempt++
	}
	return false
}

// ------------------------------
// (9) Создать затычку
// ------------------------------
func createStubConfig(domain string) {
	confpath := filepath.Join(NGINX_AVAILABLE, domain)
	stub := fmt.Sprintf(`server {
    listen 80;
    server_name %s www.%s;
    root /var/www/%s;
    index index.html index.php;

    location / {
        try_files $uri $uri/ @blank;
    }

    location @blank {
        return 200 "";
    }
}
`, domain, domain, domain)
	err := os.WriteFile(confpath, []byte(stub), 0644)
	if err != nil {
		log.Printf("[ERROR] Ошибка при создании затычки: %v", err)
		return
	}
	_ = os.Symlink(confpath, filepath.Join(NGINX_ENABLED, domain))
	_ = runCmd("nginx", "-t")
	_ = runCmd("systemctl", "reload", "nginx")
	log.Printf("[INFO] Создана затычка для %s", domain)
}

// ------------------------------
// MAIN
// ------------------------------
func main() {
	ip, err := runCmdOutput("bash", "-c", SERVER_IP_COMMAND)
	if err == nil {
		SERVER_IP = ip
	} else {
		SERVER_IP = "127.0.0.1"
	}
	TODAY = time.Now().Format("02.01.2006")
	LOG_FILE = filepath.Join(LOG_DIR, TODAY+".log")
	if err := os.MkdirAll(LOG_DIR, 0755); err != nil {
		fmt.Println("[ERROR] Не удалось создать LOG_DIR:", err)
		os.Exit(1)
	}
	f, err := os.OpenFile(LOG_FILE, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("[ERROR] Не удалось открыть LOG_FILE:", err)
		os.Exit(1)
	}
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("")
	log.Printf("[INFO] Запуск autodeploy.go; LOG_FILE=%s", LOG_FILE)

	// (A) проверка cloudflare.txt
	if !checkCloudflareFileSimple() {
		log.Println("[WARN] Данные CloudFlare не найдены или файл пуст. Продолжаем установку без интеграции CloudFlare.")
		useCloudflare = false
	}

	cmd := exec.Command("inotifywait", "-m", "-e", "create", "-e", "moved_to", WATCH_DIR)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("[ERROR] Не смогли создать StdoutPipe для inotifywait: %v", err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatalf("[ERROR] Не смогли запустить inotifywait: %v", err)
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.SplitN(line, " ", 3)
		if len(fields) < 3 {
			continue
		}
		folderName := fields[2]
		cleanOldLogs()
		log.Printf("[INFO] Обнаружена папка: %s", folderName)
		// (B) Удаление сайтов с окончанием _777
		if strings.HasSuffix(folderName, "_777") {
			realdom := strings.TrimSuffix(folderName, "_777")
			log.Printf("[INFO] Удаляем сайт %s...", realdom)
			_ = runCmd("mysql", "-u", "root", "-e", fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", realdom))
			_ = runCmd("mysql", "-u", "root", "-e", fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", realdom))
			os.RemoveAll(filepath.Join(WATCH_DIR, folderName))
			os.Remove(filepath.Join(NGINX_ENABLED, realdom))
			os.Remove(filepath.Join(NGINX_AVAILABLE, realdom))
			os.RemoveAll(filepath.Join("/etc/letsencrypt/live", realdom))
			os.RemoveAll(filepath.Join("/etc/letsencrypt/archive", realdom))
			os.Remove(filepath.Join("/etc/letsencrypt/renewal", realdom+".conf"))
			_ = runCmd("nginx", "-t")
			_ = runCmd("systemctl", "reload", "nginx")
			log.Printf("[INFO] Сайт %s успешно удалён.", realdom)
			continue
		}
		// (C) Проверка наличия статуса (idx 0..7)
		parts := strings.Split(folderName, "_")
		if len(parts) < 2 {
			log.Printf("[ERROR] Папка %s не соответствует статусам 0..7. Пропускаем.", folderName)
			continue
		}
		baseIdx := parts[len(parts)-1]
		if !strings.ContainsAny(baseIdx, "01234567") || len(baseIdx) != 1 {
			log.Printf("[ERROR] Папка %s не соответствует статусам 0..7. Пропускаем.", folderName)
			continue
		}
		// (D) Переименование папки
		realdom := strings.Join(parts[:len(parts)-1], "_")
		log.Printf("[INFO] Переименовываем %s -> %s", folderName, realdom)
		if realdom == "" {
			log.Printf("[ERROR] realdom пуст!")
			continue
		}
		oldPath := filepath.Join(WATCH_DIR, folderName)
		newPath := filepath.Join(WATCH_DIR, realdom)
		os.Rename(oldPath, newPath)
		// (E) Проверка домена через CloudFlare, если используется
		if useCloudflare {
			if !checkDomainCloudflare(realdom) {
				log.Println("[ERROR] Cloudflare ошибка!")
				suffix := getErrorSuffix(baseIdx, "cloudflare")
				newName := fmt.Sprintf("%s_%s", realdom, suffix)
				log.Printf("[INFO] Переименовываем => %s", newName)
				os.Rename(newPath, filepath.Join(WATCH_DIR, newName))
				continue
			}
		} else {
			log.Printf("[INFO] Пропускаем проверку CloudFlare для %s, т.к. данные CloudFlare не заданы.", realdom)
		}
		// (F) Установка SSL flexible через CloudFlare (если используется)
		if useCloudflare {
			setCFSSLMode("flexible")
		} else {
			log.Printf("[INFO] Пропускаем установку CloudFlare SSL (flexible) для %s.", realdom)
		}
		// (G) Создание затычки
		createStubConfig(realdom)
		// (H) Проверка 9-символьного текста
		rtext, err := generate9chars()
		if err != nil {
			log.Printf("[ERROR] Не смогли сгенерировать 9-символьный текст: %v", err)
			continue
		}
		log.Printf("[INFO] Случайный текст: %s", rtext)
		os.MkdirAll(filepath.Join("/var/www", realdom), 0755)
		indexFile := filepath.Join("/var/www", realdom, "index.php")
		os.WriteFile(indexFile, []byte(rtext), 0644)
		runCmd("chown", "-R", "www-data:www-data", filepath.Join("/var/www", realdom))
		runCmd("find", filepath.Join("/var/www", realdom), "-type", "d", "-exec", "chmod", "755", "{}", ";")
		runCmd("find", filepath.Join("/var/www", realdom), "-type", "f", "-exec", "chmod", "644", "{}", ";")
		if !checkText3Attempts(realdom, rtext) {
			log.Printf("[ERROR] Не нашли текст %s!", rtext)
			suffix := getErrorSuffix(baseIdx, "check_text")
			newName := fmt.Sprintf("%s_%s", realdom, suffix)
			os.Remove(filepath.Join(NGINX_ENABLED, realdom))
			os.Remove(filepath.Join(NGINX_AVAILABLE, realdom))
			os.Rename(filepath.Join("/var/www", realdom), filepath.Join("/var/www", newName))
			continue
		} else {
			log.Printf("[INFO] Текст найден, удаляем проверочный index.php...")
			os.Remove(indexFile)
		}
		// (I) Определяем тип сайта, необходимость SSL и использование www
		siteType := "static"
		sslNeeded := "no"
		useWww := "no"
		switch baseIdx {
		case "0":
			siteType = "static"
			sslNeeded = "no"
			useWww = "no"
		case "1":
			siteType = "static"
			sslNeeded = "no"
			useWww = "yes"
		case "2":
			siteType = "static"
			sslNeeded = "yes"
			useWww = "no"
		case "3":
			siteType = "static"
			sslNeeded = "yes"
			useWww = "yes"
		case "4":
			siteType = "wp"
			sslNeeded = "no"
			useWww = "no"
		case "5":
			siteType = "wp"
			sslNeeded = "no"
			useWww = "yes"
		case "6":
			siteType = "wp"
			sslNeeded = "yes"
			useWww = "no"
		case "7":
			siteType = "wp"
			sslNeeded = "yes"
			useWww = "yes"
		}
		log.Printf("[INFO] site_type=%s, ssl_needed=%s, domain=%s", siteType, sslNeeded, realdom)
		// (J) Генерация паролей
		dbPass, _ := runCmdOutput("bash", "-c", "openssl rand -base64 12 | tr -dc A-Za-z0-9 | head -c9")
		adminPass, _ := runCmdOutput("bash", "-c", "openssl rand -base64 12 | tr -dc A-Za-z0-9 | head -c12")
		// (K) Выбор финального шаблона
		finalTemplate := ""
		if sslNeeded == "no" && useWww == "no" {
			finalTemplate = TPL_NOSSL_NOWWW
		} else if sslNeeded == "no" && useWww == "yes" {
			finalTemplate = TPL_NOSSL_WWW
		} else if sslNeeded == "yes" && useWww == "no" {
			finalTemplate = TPL_SSL_NOWWW
		} else {
			finalTemplate = TPL_SSL_WWW
		}
		// (L) Деплой: статический сайт или WordPress
		if siteType == "static" {
			log.Println("[INFO] Статический => создаём index.php c 'IN'")
			os.WriteFile(filepath.Join("/var/www", realdom, "index.php"), []byte("<?php echo 'IN'; ?>"), 0644)
		} else {
			log.Println("[INFO] Устанавливаем WordPress...")
			os.Chdir(filepath.Join("/var/www", realdom))
			runCmd("wp", "core", "download", "--allow-root")
			runCmd("mysql", "-u", "root", "-e", fmt.Sprintf("CREATE DATABASE `%s`;", realdom))
			runCmd("mysql", "-u", "root", "-e", fmt.Sprintf("CREATE USER '%s'@'localhost' IDENTIFIED BY '%s';", realdom, dbPass))
			runCmd("mysql", "-u", "root", "-e", fmt.Sprintf("GRANT ALL ON `%s`.* TO '%s'@'localhost';", realdom, realdom))
			runCmd("mysql", "-u", "root", "-e", "FLUSH PRIVILEGES;")
			runCmd("wp", "config", "create",
				fmt.Sprintf("--dbname=%s", realdom),
				fmt.Sprintf("--dbuser=%s", realdom),
				fmt.Sprintf("--dbpass=%s", dbPass),
				"--dbhost=localhost",
				"--allow-root",
			)
			fcfg, _ := os.OpenFile("wp-config.php", os.O_APPEND|os.O_WRONLY, 0644)
			if fcfg != nil {
				fcfg.WriteString("define('FS_METHOD','direct');\n")
				fcfg.Close()
			}
			siteURL := fmt.Sprintf("https://%s", realdom)
			if useWww == "yes" {
				siteURL = fmt.Sprintf("https://www.%s", realdom)
			}
			runCmd("wp", "core", "install",
				fmt.Sprintf("--url=%s", siteURL),
				fmt.Sprintf("--title=%s Site", realdom),
				fmt.Sprintf("--admin_user=%s", realdom),
				fmt.Sprintf("--admin_password=%s", adminPass),
				fmt.Sprintf("--admin_email=admin@%s", realdom),
				"--allow-root",
			)
			fwp, _ := os.OpenFile(WP_LOG, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if fwp != nil {
				line := fmt.Sprintf("%s|%s|%s|%s|%s\n", realdom, realdom, adminPass, realdom, dbPass)
				fwp.WriteString(line)
				fwp.Close()
			}
		}
		runCmd("chown", "-R", "www-data:www-data", filepath.Join("/var/www", realdom))
		runCmd("find", filepath.Join("/var/www", realdom), "-type", "d", "-exec", "chmod", "755", "{}", ";")
		runCmd("find", filepath.Join("/var/www", realdom), "-type", "f", "-exec", "chmod", "644", "{}", ";")
		// (M) Если нужен SSL, запускаем certbot и обновляем конфигурацию
		if sslNeeded == "yes" {
			log.Printf("[INFO] Выпускаем SSL (certbot) для %s...", realdom)
			errC := runCmd("certbot", "--nginx", "-d", realdom, "--non-interactive", "--agree-tos", "-m", fmt.Sprintf("admin@%s", realdom))
			if errC == nil {
				log.Println("[INFO] SSL выпущен => убираем затычку, ставим финальный SSL, CF=full")
				os.Remove(filepath.Join(NGINX_ENABLED, realdom))
				os.Remove(filepath.Join(NGINX_AVAILABLE, realdom))
				newConf := filepath.Join(NGINX_AVAILABLE, realdom)
				dataTempl, errF := os.ReadFile(finalTemplate)
				if errF == nil {
					confText := strings.ReplaceAll(string(dataTempl), "{{ domain_name }}", realdom)
					os.WriteFile(newConf, []byte(confText), 0644)
				}
				os.Symlink(newConf, filepath.Join(NGINX_ENABLED, realdom))
				runCmd("nginx", "-t")
				runCmd("systemctl", "reload", "nginx")
				if useCloudflare {
					setCFSSLMode("full")
				} else {
					log.Printf("[INFO] Пропускаем установку CloudFlare SSL (full) для %s.", realdom)
				}
			} else {
				log.Println("[ERROR] Ошибка SSL!")
				suffix := getErrorSuffix(baseIdx, "other")
				newName := fmt.Sprintf("%s_%s", realdom, suffix)
				os.Remove(filepath.Join(NGINX_ENABLED, realdom))
				os.Remove(filepath.Join(NGINX_AVAILABLE, realdom))
				os.RemoveAll(filepath.Join("/var/www", realdom))
				os.RemoveAll(filepath.Join("/etc/letsencrypt/live", realdom))
				os.RemoveAll(filepath.Join("/etc/letsencrypt/archive", realdom))
				os.Remove(filepath.Join("/etc/letsencrypt/renewal", realdom+".conf"))
				os.Mkdir(filepath.Join(WATCH_DIR, newName), 0755)
				runCmd("nginx", "-t")
				runCmd("systemctl", "reload", "nginx")
				continue
			}
		} else {
			log.Println("[INFO] SSL не нужен => убираем затычку, ставим final_template, CF=flexible")
			os.Remove(filepath.Join(NGINX_ENABLED, realdom))
			os.Remove(filepath.Join(NGINX_AVAILABLE, realdom))
			newConf := filepath.Join(NGINX_AVAILABLE, realdom)
			dataTempl, errF := os.ReadFile(finalTemplate)
			if errF == nil {
				confText := strings.ReplaceAll(string(dataTempl), "{{ domain_name }}", realdom)
				os.WriteFile(newConf, []byte(confText), 0644)
			}
			os.Symlink(newConf, filepath.Join(NGINX_ENABLED, realdom))
			runCmd("nginx", "-t")
			runCmd("systemctl", "reload", "nginx")
			if useCloudflare {
				setCFSSLMode("flexible")
			} else {
				log.Printf("[INFO] Пропускаем установку CloudFlare SSL (flexible) для %s.", realdom)
			}
		}
		// (N) Применяем дефолтные настройки CloudFlare, если используется
		if useCloudflare {
			log.Println("[INFO] Применяем финальные дефолтные настройки CF...")
			applyDefaultCFSettings()
		} else {
			log.Println("[INFO] CloudFlare не настроен, пропускаем применение настроек CF.")
		}
		log.Printf("[INFO] Сайт %s развернут успешно.", realdom)
	}
	if err := cmd.Wait(); err != nil {
		log.Printf("[ERROR] inotifywait завершился с ошибкой: %v", err)
	}
}
