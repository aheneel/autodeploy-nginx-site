package main

import (
    "crypto/rand"
    "fmt"
    "log"
    "math/big"
    "os"
    "os/exec"
    "strings"
)

// runCommand выполняет команду cmd[0] с аргументами cmd[1:], 
// перенаправляя stdout и stderr в консоль (в реальном времени).
// При ошибке возвращает error, что приводит к прерыванию (log.Fatalf) в main().
func runCommand(cmd []string) error {
    if len(cmd) == 0 {
        return fmt.Errorf("пустая команда")
    }
    command := exec.Command(cmd[0], cmd[1:]...)
    command.Stdout = os.Stdout
    command.Stderr = os.Stderr

    if err := command.Run(); err != nil {
        return fmt.Errorf("команда '%s' вернула ошибку: %v",
            strings.Join(cmd, " "), err)
    }
    return nil
}

// generateRandomString генерирует случайную строку длиной n символов
// из набора [A-Za-z0-9] при помощи криптографически стойкого генератора.
func generateRandomString(n int) (string, error) {
    const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
    result := make([]byte, n)
    bigLen := big.NewInt(int64(len(chars)))

    for i := 0; i < n; i++ {
        num, err := rand.Int(rand.Reader, bigLen)
        if err != nil {
            return "", err
        }
        result[i] = chars[num.Int64()]
    }
    return string(result), nil
}

// step1CreateSSLCert - Шаг 1:
// 1. Создаёт директории /etc/ssl/private и /etc/ssl/certs
// 2. Генерирует самоподписанный SSL-сертификат (на 365 дней)
func step1CreateSSLCert() error {
    log.Println("[Шаг 1] Создаю директории /etc/ssl/private и /etc/ssl/certs...")
    if err := runCommand([]string{"mkdir", "-p", "/etc/ssl/private", "/etc/ssl/certs"}); err != nil {
        return err
    }

    log.Println("   Генерирую самоподписанный SSL-сертификат (на 365 дней)...")
    // Аналог команды:
    // openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    //   -keyout /etc/ssl/private/default.key \
    //   -out /etc/ssl/certs/default.crt \
    //   -subj "/C=US/ST=Test/L=Test/O=Test/OU=Test/CN=example.com"
    cmd := []string{
        "openssl", "req",
        "-x509", "-nodes", "-days", "365", "-newkey", "rsa:2048",
        "-keyout", "/etc/ssl/private/default.key",
        "-out", "/etc/ssl/certs/default.crt",
        "-subj", "/C=US/ST=Test/L=Test/O=Test/OU=Test/CN=example.com",
    }
    if err := runCommand(cmd); err != nil {
        return err
    }

    return nil
}

// step2GenerateRandomPathAndWriteDefault - Шаг 2:
// 1. Генерирует уникальную 19-символьную строку (Go-методом!).
// 2. Формирует /etc/nginx/sites-available/default c подстановкой этого пути.
func step2GenerateRandomPathAndWriteDefault() (string, error) {
    log.Println("[Шаг 2] Генерирую уникальную 19-символьную строку для phpMyAdmin (Go-методом)...")
    randomPath, err := generateRandomString(19)
    if err != nil {
        return "", fmt.Errorf("не удалось сгенерировать строку: %v", err)
    }
    log.Printf("   Сгенерированный путь: %s\n", randomPath)

    log.Println("   Формирую /etc/nginx/sites-available/default...")

    // Шаблон для /etc/nginx/sites-available/default
    // Заменяем $random_path на значение randomPath
    // Обратите внимание, что в shell-скрипте было 521, слушаем порты 80 и 443, etc.
    confContent := fmt.Sprintf(`
server {
    listen 80 default_server;
    server_name _;

    root /dev/null;

    location / {
        return 521;
    }

    location /%s {
        alias /usr/share/phpmyadmin/;
        index index.php index.html;

        location ~ \.php$ {
            include snippets/fastcgi-php.conf;
            fastcgi_param SCRIPT_FILENAME $request_filename;
            fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        }
    }
}

server {
    listen 443 ssl default_server;
    server_name _;

    ssl_certificate /etc/ssl/certs/default.crt;
    ssl_certificate_key /etc/ssl/private/default.key;

    root /dev/null;

    location / {
        return 521;
    }

    location /%s {
        alias /usr/share/phpmyadmin/;
        index index.php index.html;

        location ~ \.php$ {
            include snippets/fastcgi-php.conf;
            fastcgi_param SCRIPT_FILENAME $request_filename;
            fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        }
    }
}
`, randomPath, randomPath)

    // Записываем полученный конфиг в файл
    if err := os.WriteFile("/etc/nginx/sites-available/default", []byte(confContent), 0644); err != nil {
        return "", fmt.Errorf("не удалось записать /etc/nginx/sites-available/default: %v", err)
    }

    return randomPath, nil
}

// step3CheckNginxConfig - Шаг 3: Проверяем конфигурацию Nginx (nginx -t).
func step3CheckNginxConfig() error {
    log.Println("[Шаг 3] Проверяю конфигурацию nginx...")
    return runCommand([]string{"nginx", "-t"})
}

// step4ReloadNginx - Шаг 4: Перезагружаем Nginx.
func step4ReloadNginx() error {
    log.Println("[Шаг 4] Перезагружаю Nginx...")
    return runCommand([]string{"systemctl", "restart", "nginx"})
}

// step5SaveRandomPath - Шаг 5: Записываем путь в /root/auto_deploy/phpMyAdmin.txt.
func step5SaveRandomPath(randomPath string) error {
    log.Println("[Шаг 5] Записываю путь в /root/auto_deploy/phpMyAdmin.txt...")
    if err := runCommand([]string{"mkdir", "-p", "/root/auto_deploy"}); err != nil {
        return err
    }

    // append (>>) "/$randomPath/"
    // эквивалентно: echo "/$random_path/" >> /root/auto_deploy/phpMyAdmin.txt
    f, err := os.OpenFile("/root/auto_deploy/phpMyAdmin.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return fmt.Errorf("не удалось открыть /root/auto_deploy/phpMyAdmin.txt: %v", err)
    }
    defer f.Close()

    line := fmt.Sprintf("/%s/\n", randomPath)
    if _, err := f.WriteString(line); err != nil {
        return fmt.Errorf("не удалось записать в phpMyAdmin.txt: %v", err)
    }

    return nil
}

// step6CheckNginxAgain - Шаг 6: Снова проверяем конфигурацию nginx.
func step6CheckNginxAgain() error {
    log.Println("[Шаг 6] Снова проверяю конфигурацию nginx...")
    return runCommand([]string{"nginx", "-t"})
}

// step7ReloadNginxAgain - Шаг 7: Снова перезагружаем Nginx.
func step7ReloadNginxAgain() error {
    log.Println("[Шаг 7] Снова перезагружаю Nginx...")
    return runCommand([]string{"systemctl", "restart", "nginx"})
}

func main() {
    // ========== Шаг 1 ==========
    if err := step1CreateSSLCert(); err != nil {
        log.Fatalf("[Ошибка на шаге 1]: %v", err)
    }
    log.Println("[Шаг 1] выполнен успешно.")

    // ========== Шаг 2 ==========
    randomPath, err := step2GenerateRandomPathAndWriteDefault()
    if err != nil {
        log.Fatalf("[Ошибка на шаге 2]: %v", err)
    }
    log.Println("[Шаг 2] выполнен успешно.")

    // ========== Шаг 3 ==========
    if err := step3CheckNginxConfig(); err != nil {
        log.Fatalf("[Ошибка на шаге 3]: %v", err)
    }
    log.Println("[Шаг 3] выполнен успешно.")

    // ========== Шаг 4 ==========
    if err := step4ReloadNginx(); err != nil {
        log.Fatalf("[Ошибка на шаге 4]: %v", err)
    }
    log.Println("[Шаг 4] выполнен успешно.")

    // ========== Шаг 5 ==========
    if err := step5SaveRandomPath(randomPath); err != nil {
        log.Fatalf("[Ошибка на шаге 5]: %v", err)
    }
    log.Println("[Шаг 5] выполнен успешно.")

    // ========== Шаг 6 ==========
    if err := step6CheckNginxAgain(); err != nil {
        log.Fatalf("[Ошибка на шаге 6]: %v", err)
    }
    log.Println("[Шаг 6] выполнен успешно.")

    // ========== Шаг 7 ==========
    if err := step7ReloadNginxAgain(); err != nil {
        log.Fatalf("[Ошибка на шаге 7]: %v", err)
    }
    log.Println("[Шаг 7] выполнен успешно.")

    // Если дошли сюда, значит всё прошло без ошибок
    log.Println("Все шаги выполнены успешно!")
}
