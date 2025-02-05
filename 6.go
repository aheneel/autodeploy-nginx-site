package main

import (
    "fmt"
    "io"
    "log"
    "os"
    "os/exec"
    "strings"
)

// runCommand запускает указанную команду (cmd[0]) с аргументами (cmd[1:]).
// Stdout и stderr перенаправляются в консоль.
// При ошибке возвращает error.
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

// copyFile копирует исходный файл (src) в целевой файл (dst).
// Если dst уже существует, будет перезаписан.
func copyFile(src, dst string) error {
    in, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("не удалось открыть исходный файл '%s': %v", src, err)
    }
    defer in.Close()

    out, err := os.Create(dst)
    if err != nil {
        return fmt.Errorf("не удалось создать файл '%s': %v", dst, err)
    }
    defer out.Close()

    if _, err = io.Copy(out, in); err != nil {
        return fmt.Errorf("ошибка копирования из '%s' в '%s': %v", src, dst, err)
    }

    return nil
}

// Шаг 1. Скопировать бинарник /root/install/autodeploy в /root/auto_deploy/,
//        затем сделать /root/auto_deploy исполняемым (chmod -R +x).
func step1CopyBinaryAndChmod() error {
    log.Println("[Шаг 1] Копирую /root/install/autodeploy в /root/auto_deploy/autodeploy...")

    // Допустим, что исходный файл находится именно по пути /root/install/autodeploy
    // и мы хотим скопировать его в /root/auto_deploy/autodeploy
    if err := copyFile("/root/install/autodeploy", "/root/auto_deploy/autodeploy"); err != nil {
        return err
    }

    log.Println("   Делаю /root/auto_deploy/ исполняемым...")
    // chmod -R +x /root/auto_deploy
    return runCommand([]string{"chmod", "-R", "+x", "/root/auto_deploy"})
}

// Шаг 2. Создать /etc/systemd/system/autodeploy.service, daemon-reload, enable, start
func step2CreateAutodeployService() error {
    log.Println("[Шаг 2] Создаю /etc/systemd/system/autodeploy.service...")

    serviceContent := `[Unit]
Description=Auto Deploy Nginx Sites on Folder Creation
After=network.target mariadb.service nginx.service

[Service]
ExecStart=/root/auto_deploy/autodeploy
Restart=always
RestartSec=5
User=root
Group=root
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`
    // Пишем файл
    if err := os.WriteFile("/etc/systemd/system/autodeploy.service",
        []byte(serviceContent), 0644); err != nil {
        return fmt.Errorf("не удалось записать /etc/systemd/system/autodeploy.service: %v", err)
    }

    log.Println("   Перезагружаю конфигурацию systemd...")
    if err := runCommand([]string{"systemctl", "daemon-reload"}); err != nil {
        return err
    }

    log.Println("   Включаю и запускаю autodeploy.service...")
    if err := runCommand([]string{"systemctl", "enable", "autodeploy.service"}); err != nil {
        return err
    }
    if err := runCommand([]string{"systemctl", "start", "autodeploy.service"}); err != nil {
        return err
    }

    return nil
}

// Шаг 3. Создать /etc/systemd/system/certbot-renew.service
func step3CreateCertbotRenewService() error {
    log.Println("[Шаг 3] Создаю /etc/systemd/system/certbot-renew.service...")

    serviceContent := `[Unit]
Description=Certbot Renew Service
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/bin/certbot renew --post-hook "systemctl reload nginx"
`
    if err := os.WriteFile("/etc/systemd/system/certbot-renew.service",
        []byte(serviceContent), 0644); err != nil {
        return fmt.Errorf("не удалось записать /etc/systemd/system/certbot-renew.service: %v", err)
    }
    return nil
}

// Шаг 4. Создать /etc/systemd/system/certbot-renew.timer, потом daemon-reload, enable, start
func step4CreateCertbotRenewTimer() error {
    log.Println("[Шаг 4] Создаю /etc/systemd/system/certbot-renew.timer...")

    timerContent := `[Unit]
Description=Run certbot renew every day

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
`
    if err := os.WriteFile("/etc/systemd/system/certbot-renew.timer",
        []byte(timerContent), 0644); err != nil {
        return fmt.Errorf("не удалось записать /etc/systemd/system/certbot-renew.timer: %v", err)
    }

    log.Println("   Перезагружаю конфигурацию systemd (повторно)...")
    if err := runCommand([]string{"systemctl", "daemon-reload"}); err != nil {
        return err
    }

    log.Println("   Включаю и запускаю certbot-renew.timer...")
    if err := runCommand([]string{"systemctl", "enable", "certbot-renew.timer"}); err != nil {
        return err
    }
    if err := runCommand([]string{"systemctl", "start", "certbot-renew.timer"}); err != nil {
        return err
    }

    return nil
}

// Шаг 5. Рестартуем php8.2-fpm, mariadb, nginx, autodeploy.service, certbot-renew.timer
// и выводим сообщение "Установка успешно закончена."
func step5RestartServices() error {
    log.Println("[Шаг 5] Перезапускаю php8.2-fpm, mariadb, nginx, autodeploy.service, certbot-renew.timer...")

    cmds := [][]string{
        {"systemctl", "restart", "php8.2-fpm"},
        {"systemctl", "restart", "mariadb"},
        {"systemctl", "restart", "nginx"},
        {"systemctl", "restart", "autodeploy.service"},
        {"systemctl", "restart", "certbot-renew.timer"},
    }

    for _, cmd := range cmds {
        if err := runCommand(cmd); err != nil {
            return err
        }
    }

    // Выводим завершающее сообщение
    log.Println("Установка успешно закончена.")
    return nil
}

func main() {
    steps := []func() error{
        step1CopyBinaryAndChmod,
        step2CreateAutodeployService,
        step3CreateCertbotRenewService,
        step4CreateCertbotRenewTimer,
        step5RestartServices,
    }

    for i, step := range steps {
        stepNumber := i + 1
        log.Printf("[Шаг %d] Начало...\n", stepNumber)
        if err := step(); err != nil {
            log.Fatalf("[Ошибка на шаге %d]: %v", stepNumber, err)
        }
        log.Printf("[Шаг %d] выполнен успешно.\n", stepNumber)
    }

    // Если дошли сюда - всё успешно
    // (Финальное сообщение уже было выведено в шаге 5)
}
