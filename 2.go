package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "strings"
)

// Константы с путями к файлам, аналогичные вашему 2.sh
const (
    PHP_INI    = "/etc/php/8.2/fpm/php.ini"
    NGINX_CONF = "/etc/nginx/nginx.conf"
)

// runCommand запускает команду (cmd[0]) с аргументами (cmd[1:]...).
// Если stdinData != "", оно передаётся в stdin процесса.
// Stdout и stderr команды отображаются **в реальном времени** в консоли.
// При ошибке возвращает error.
func runCommand(cmd []string, stdinData string) error {
    if len(cmd) == 0 {
        return fmt.Errorf("пустая команда")
    }
    command := exec.Command(cmd[0], cmd[1:]...)

    // Если нужно отправить что-то в stdin
    if stdinData != "" {
        command.Stdin = strings.NewReader(stdinData)
    }

    // Перенаправляем вывод команды прямо в консоль:
    command.Stdout = os.Stdout
    command.Stderr = os.Stderr

    if err := command.Run(); err != nil {
        return fmt.Errorf("команда '%s' вернула ошибку: %v",
            strings.Join(cmd, " "), err)
    }
    return nil
}

// Шаг 1. Удаляем строки post_max_size
func step1RemovePostMaxSize() error {
    log.Println("[Шаг 1] Удаляю строки, где есть 'post_max_size = '...")
    // sed -i -E '/^\s*(; )?post_max_size = /d' "$PHP_INI"
    if err := runCommand([]string{
        "sed", "-i", "-E", `/^\s*(; )?post_max_size = /d`, PHP_INI,
    }, ""); err != nil {
        return err
    }
    log.Println("[Шаг 1] выполнен успешно.")
    return nil
}

// Шаг 2. Удаляем строки upload_max_filesize
func step2RemoveUploadMaxFilesize() error {
    log.Println("[Шаг 2] Удаляю строки, где есть 'upload_max_filesize = '...")
    // sed -i -E '/^\s*(; )?upload_max_filesize = /d' "$PHP_INI"
    if err := runCommand([]string{
        "sed", "-i", "-E", `/^\s*(; )?upload_max_filesize = /d`, PHP_INI,
    }, ""); err != nil {
        return err
    }
    log.Println("[Шаг 2] выполнен успешно.")
    return nil
}

// Шаг 3. Удаляем строки max_input_vars
func step3RemoveMaxInputVars() error {
    log.Println("[Шаг 3] Удаляю строки, где есть 'max_input_vars = '...")
    // sed -i -E '/^\s*(; )?max_input_vars = /d' "$PHP_INI"
    if err := runCommand([]string{
        "sed", "-i", "-E", `/^\s*(; )?max_input_vars = /d`, PHP_INI,
    }, ""); err != nil {
        return err
    }
    log.Println("[Шаг 3] выполнен успешно.")
    return nil
}

// Шаг 4. Удаляем строки memory_limit
func step4RemoveMemoryLimit() error {
    log.Println("[Шаг 4] Удаляю строки, где есть 'memory_limit = '...")
    // sed -i -E '/^\s*(; )?memory_limit = /d' "$PHP_INI"
    if err := runCommand([]string{
        "sed", "-i", "-E", `/^\s*(; )?memory_limit = /d`, PHP_INI,
    }, ""); err != nil {
        return err
    }
    log.Println("[Шаг 4] выполнен успешно.")
    return nil
}

// Шаг 5. Добавляем новые значения в php.ini
func step5AddNewValuesToPHPINI() error {
    log.Println("[Шаг 5] Добавляю новые значения в", PHP_INI, "...")

    // echo "post_max_size = 128M" >> "$PHP_INI"
    if err := runCommand([]string{"sh", "-c",
        fmt.Sprintf(`echo "post_max_size = 128M" >> "%s"`, PHP_INI)}, ""); err != nil {
        return err
    }
    // echo "upload_max_filesize = 128M" >> "$PHP_INI"
    if err := runCommand([]string{"sh", "-c",
        fmt.Sprintf(`echo "upload_max_filesize = 128M" >> "%s"`, PHP_INI)}, ""); err != nil {
        return err
    }
    // echo "max_input_vars = 1000" >> "$PHP_INI"
    if err := runCommand([]string{"sh", "-c",
        fmt.Sprintf(`echo "max_input_vars = 1000" >> "%s"`, PHP_INI)}, ""); err != nil {
        return err
    }
    // echo "memory_limit = 128M" >> "$PHP_INI"
    if err := runCommand([]string{"sh", "-c",
        fmt.Sprintf(`echo "memory_limit = 128M" >> "%s"`, PHP_INI)}, ""); err != nil {
        return err
    }

    log.Println("[Шаг 5] выполнен успешно.")
    return nil
}

// Шаг 6. Перезапускаем php8.2-fpm
func step6RestartPHPFPM() error {
    log.Println("[Шаг 6] Перезапускаю php8.2-fpm...")
    if err := runCommand([]string{"systemctl", "restart", "php8.2-fpm"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 6] выполнен успешно.")
    return nil
}

// Шаг 7. Добавляем 'client_max_body_size 128M;' в http { ... } NGINX
func step7AddClientMaxBodySize() error {
    log.Println("[Шаг 7] Ищу 'http {' и добавляю строку 'client_max_body_size 128M;'...")
    // sed -i '/http {/a \        client_max_body_size 128M;' "$NGINX_CONF"
    if err := runCommand([]string{
        "sed", "-i", `/http {/a \        client_max_body_size 128M;`, NGINX_CONF,
    }, ""); err != nil {
        return err
    }
    log.Println("[Шаг 7] выполнен успешно.")
    return nil
}

// Шаг 8. Проверяем конфигурацию Nginx
func step8CheckNginxConfig() error {
    log.Println("[Шаг 8] Проверяю конфигурацию Nginx (nginx -t)...")
    if err := runCommand([]string{"nginx", "-t"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 8] выполнен успешно.")
    return nil
}

// Шаг 9. Перезапускаем Nginx
func step9RestartNginx() error {
    log.Println("[Шаг 9] Перезапускаю Nginx...")
    if err := runCommand([]string{"systemctl", "restart", "nginx"}, ""); err != nil {
        return err
    }
    log.Println("[Шаг 9] выполнен успешно.")
    return nil
}

func main() {
    steps := []func() error{
        step1RemovePostMaxSize,
        step2RemoveUploadMaxFilesize,
        step3RemoveMaxInputVars,
        step4RemoveMemoryLimit,
        step5AddNewValuesToPHPINI,
        step6RestartPHPFPM,
        step7AddClientMaxBodySize,
        step8CheckNginxConfig,
        step9RestartNginx,
    }

    // Выполняем шаги по порядку
    for i, step := range steps {
        if err := step(); err != nil {
            log.Fatalf("[Ошибка на шаге %d]: %v", i+1, err)
        }
    }

    log.Println("Все шаги выполнены успешно!")
}
