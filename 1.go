package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "strings"
)

// runCommand выполняет cmd[0] с аргументами cmd[1:].
// Если stdinData != "", отправляет её в stdin процесса.
// Потоки stdout/stderr команды перенаправляются непосредственно в stdout/stderr Go-приложения,
// чтобы вы видели процесс установки в реальном времени.
// Если команда завершается с ошибкой, возвращаем err.
func runCommand(cmd []string, stdinData string) error {
    if len(cmd) == 0 {
        return fmt.Errorf("пустая команда")
    }

    command := exec.Command(cmd[0], cmd[1:]...)

    // Если нужно что-то передать в stdin
    if stdinData != "" {
        command.Stdin = strings.NewReader(stdinData)
    }

    // Перенаправляем вывод команды прямо в консоль
    command.Stdout = os.Stdout
    command.Stderr = os.Stderr

    // Запускаем команду и ждём её завершения
    if err := command.Run(); err != nil {
        // Вывод команды уже был выведен на экран,
        // поэтому здесь можно просто вернуть ошибку без дампа.
        return fmt.Errorf("команда '%s' вернула ошибку: %v",
            strings.Join(cmd, " "), err)
    }
    return nil
}

// Шаг 1. Устанавливаем переменные окружения для неинтерактивного режима.
func step1SetEnv() error {
    log.Println("[Шаг 1] Устанавливаем переменные окружения DEBIAN_FRONTEND=noninteractive...")
    if err := os.Setenv("DEBIAN_FRONTEND", "noninteractive"); err != nil {
        return fmt.Errorf("не удалось установить DEBIAN_FRONTEND: %v", err)
    }
    log.Println("[Шаг 1] выполнен успешно.")
    return nil
}

// Шаг 2. apt-get update.
func step2AptUpdate() error {
    log.Println("[Шаг 2] Обновляем списки пакетов (apt-get update)...")
    // -yq скрывает часть вывода; если хотите видеть больше, уберите -q
    if err := runCommand([]string{"apt-get", "update", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 2] выполнен успешно.")
    return nil
}

// Шаг 3. apt-get upgrade.
func step3AptUpgrade() error {
    log.Println("[Шаг 3] Устанавливаем доступные обновления (apt-get upgrade)...")
    if err := runCommand([]string{"apt-get", "upgrade", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 3] выполнен успешно.")
    return nil
}

// Шаг 4. Отключаем выбор веб-сервера при установке phpmyadmin.
func step4Debconf() error {
    log.Println("[Шаг 4] Отключаю выбор веб-сервера при установке phpmyadmin...")

    // На случай, если debconf-utils не установлен:
    if err := runCommand([]string{"apt-get", "install", "debconf-utils", "-yq"}, ""); err != nil {
        return fmt.Errorf("не удалось установить debconf-utils: %v", err)
    }

    // Передаём строку через stdin в debconf-set-selections
    stdinData := "phpmyadmin phpmyadmin/reconfigure-webserver multiselect none"
    if err := runCommand([]string{"debconf-set-selections"}, stdinData); err != nil {
        return fmt.Errorf("Отключение веб-сервера для phpMyAdmin: %v", err)
    }
    log.Println("[Шаг 4] выполнен успешно.")
    return nil
}

// Шаг 5. Снова выставляем неинтерактивный режим на всякий случай.
func step5SetEnvAgain() error {
    log.Println("[Шаг 5] Снова выставляем неинтерактивный режим...")
    if err := os.Setenv("DEBIAN_FRONTEND", "noninteractive"); err != nil {
        return fmt.Errorf("не удалось установить переменную окружения DEBIAN_FRONTEND: %v", err)
    }
    log.Println("[Шаг 5] выполнен успешно.")
    return nil
}

// Шаг 6. Устанавливаю nginx.
func step6InstallNginx() error {
    log.Println("[Шаг 6] Устанавливаю nginx...")
    if err := runCommand([]string{"apt-get", "install", "nginx", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 6] выполнен успешно.")
    return nil
}

// Шаг 7. Устанавливаю php8.2-fpm.
func step7InstallPHPFPM() error {
    log.Println("[Шаг 7] Устанавливаю php8.2-fpm...")
    if err := runCommand([]string{"apt-get", "install", "php8.2-fpm", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 7] выполнен успешно.")
    return nil
}

// Шаг 8. Устанавливаю php8.2-mysql.
func step8InstallPHPMysql() error {
    log.Println("[Шаг 8] Устанавливаю php8.2-mysql...")
    if err := runCommand([]string{"apt-get", "install", "php8.2-mysql", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 8] выполнен успешно.")
    return nil
}

// Шаг 9. Устанавливаю mariadb-server.
func step9InstallMariaDB() error {
    log.Println("[Шаг 9] Устанавливаю mariadb-server...")
    if err := runCommand([]string{"apt-get", "install", "mariadb-server", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 9] выполнен успешно.")
    return nil
}

// Шаг 10. Устанавливаю inotify-tools.
func step10InstallInotifyTools() error {
    log.Println("[Шаг 10] Устанавливаю inotify-tools...")
    if err := runCommand([]string{"apt-get", "install", "inotify-tools", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 10] выполнен успешно.")
    return nil
}

// Шаг 11. Устанавливаю certbot.
func step11InstallCertbot() error {
    log.Println("[Шаг 11] Устанавливаю certbot...")
    if err := runCommand([]string{"apt-get", "install", "certbot", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 11] выполнен успешно.")
    return nil
}

// Шаг 12. Устанавливаю python3-certbot-nginx.
func step12InstallCertbotNginx() error {
    log.Println("[Шаг 12] Устанавливаю python3-certbot-nginx...")
    if err := runCommand([]string{"apt-get", "install", "python3-certbot-nginx", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 12] выполнен успешно.")
    return nil
}

// Шаг 13. Устанавливаю phpmyadmin.
func step13InstallPHPMyAdmin() error {
    log.Println("[Шаг 13] Устанавливаю phpmyadmin...")
    if err := runCommand([]string{"apt-get", "install", "phpmyadmin", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 13] выполнен успешно.")
    return nil
}

// Шаг 14. Устанавливаю curl.
func step14InstallCurl() error {
    log.Println("[Шаг 14] Устанавливаю curl...")
    if err := runCommand([]string{"apt-get", "install", "curl", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 14] выполнен успешно.")
    return nil
}

// Шаг 15. Устанавливаю jq.
func step15InstallJQ() error {
    log.Println("[Шаг 15] Устанавливаю jq...")
    if err := runCommand([]string{"apt-get", "install", "jq", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 15] выполнен успешно.")
    return nil
}

// Шаг 16. Устанавливаю openssl.
func step16InstallOpenssl() error {
    log.Println("[Шаг 16] Устанавливаю openssl...")
    if err := runCommand([]string{"apt-get", "install", "openssl", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 16] выполнен успешно.")
    return nil
}

// Шаг 17. Устанавливаю dos2unix.
func step17InstallDos2Unix() error {
    log.Println("[Шаг 17] Устанавливаю dos2unix...")
    if err := runCommand([]string{"apt-get", "install", "dos2unix", "-yq"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 17] выполнен успешно.")
    return nil
}

// Шаг 18. Устанавливаю wp-cli.
func step18InstallWPCLI() error {
    log.Println("[Шаг 18] Устанавливаю wp-cli...")

    // 1. Скачиваем wp-cli.phar
    if err := runCommand([]string{"curl", "-O", "https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar"}, ""); err != nil {
        return fmt.Errorf("не удалось скачать wp-cli.phar: %v", err)
    }

    // 2. Делаем файл исполняемым
    if err := runCommand([]string{"chmod", "+x", "wp-cli.phar"}, ""); err != nil {
        return fmt.Errorf("не удалось chmod +x wp-cli.phar: %v", err)
    }

    // 3. Переносим в /usr/local/bin/wp
    if err := runCommand([]string{"mv", "wp-cli.phar", "/usr/local/bin/wp"}, ""); err != nil {
        return fmt.Errorf("не удалось переместить wp-cli.phar: %v", err)
    }

    // 4. Проверяем wp --info
    if err := runCommand([]string{"wp", "--info"}, ""); err != nil {
        return fmt.Errorf("wp --info ошибка: %v", err)
    }

    log.Println("[Шаг 18] выполнен успешно.")
    return nil
}

func main() {
    // Список шагов
    steps := []func() error{
        step1SetEnv,
        step2AptUpdate,
        step3AptUpgrade,
        step4Debconf,
        step5SetEnvAgain,
        step6InstallNginx,
        step7InstallPHPFPM,
        step8InstallPHPMysql,
        step9InstallMariaDB,
        step10InstallInotifyTools,
        step11InstallCertbot,
        step12InstallCertbotNginx,
        step13InstallPHPMyAdmin,
        step14InstallCurl,
        step15InstallJQ,
        step16InstallOpenssl,
        step17InstallDos2Unix,
        step18InstallWPCLI,
    }

    // Выполняем шаги последовательно. При любой ошибке программа немедленно завершается.
    for i, step := range steps {
        if err := step(); err != nil {
            log.Fatalf("[Ошибка на шаге %d]: %v", i+1, err)
        }
    }

    log.Println("Все шаги выполнены успешно!")
}
