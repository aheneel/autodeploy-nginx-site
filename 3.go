package main

import (
    "bytes"
    "crypto/rand"
    "fmt"
    "log"
    "math/big"
    "os"
    "os/exec"
    "strings"
)

// runCommand выполняет команду cmd[0] с аргументами cmd[1:] и отображает stdout/stderr
// в реальном времени. При ошибке возвращает error, что прекращает выполнение.
func runCommand(cmd []string, stdinData string) error {
    if len(cmd) == 0 {
        return fmt.Errorf("пустая команда")
    }
    command := exec.Command(cmd[0], cmd[1:]...)

    if stdinData != "" {
        command.Stdin = strings.NewReader(stdinData)
    }

    command.Stdout = os.Stdout
    command.Stderr = os.Stderr

    if err := command.Run(); err != nil {
        return fmt.Errorf("команда '%s' вернула ошибку: %v",
            strings.Join(cmd, " "), err)
    }
    return nil
}

// runCommandOutput запускает команду и возвращает её stdout как строку.
// stderr при этом идёт в консоль.
func runCommandOutput(cmd []string) (string, error) {
    if len(cmd) == 0 {
        return "", fmt.Errorf("пустая команда (output)")
    }
    command := exec.Command(cmd[0], cmd[1:]...)

    // Ошибки выводим в консоль
    command.Stderr = os.Stderr

    var stdoutBuf bytes.Buffer
    command.Stdout = &stdoutBuf

    if err := command.Run(); err != nil {
        return "", fmt.Errorf("команда '%s' вернула ошибку: %v",
            strings.Join(cmd, " "), err)
    }
    return stdoutBuf.String(), nil
}

const (
    // Файлы и пути, как в 3.sh
    mariadbConf = "/etc/mysql/mariadb.conf.d/50-server.cnf"
    autoDeploy  = "/root/auto_deploy/phpMyAdmin.txt"
)

// generateRandomString генерирует случайную строку длиной n символов
// из набора [A-Za-z0-9], используя криптографический генератор случайных чисел.
func generateRandomString(n int) (string, error) {
    const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
    result := make([]byte, n)

    // Длина алфавита
    bigLen := big.NewInt(int64(len(chars)))

    for i := 0; i < n; i++ {
        // Получаем случайное число в диапазоне [0, len(chars))
        num, err := rand.Int(rand.Reader, bigLen)
        if err != nil {
            return "", err
        }
        // Индексируем нужный символ
        result[i] = chars[num.Int64()]
    }

    return string(result), nil
}

// =============================================
// Шаги скрипта 3.sh
// =============================================

// Шаг 1. Удаляем тестовую базу и анонимных пользователей в MySQL.
func step1RemoveTestDBAndAnonymous() error {
    log.Println("[Шаг 1] Удаляю тестовую базу и анонимных пользователей в MySQL...")
    // sudo mysql -u root -e "DROP DATABASE IF EXISTS test; DELETE FROM mysql.user WHERE User=''; FLUSH PRIVILEGES;"
    return runCommand([]string{
        "sudo", "mysql", "-u", "root",
        "-e", "DROP DATABASE IF EXISTS test; DELETE FROM mysql.user WHERE User=''; FLUSH PRIVILEGES;",
    }, "")
}

// Шаг 2. Перезапускаю mariadb...
func step2RestartMariaDB() error {
    log.Println("[Шаг 2] Перезапускаю mariadb...")
    // systemctl restart mariadb
    return runCommand([]string{"systemctl", "restart", "mariadb"}, "")
}

// Шаг 3. Настраиваю bind-address
func step3SetBindAddress() error {
    log.Println("[Шаг 3] Настраиваю bind-address в", mariadbConf, "...")

    // sed -i '/bind-address/d' /etc/mysql/mariadb.conf.d/50-server.cnf
    if err := runCommand([]string{
        "sed", "-i", "/bind-address/d", mariadbConf,
    }, ""); err != nil {
        return err
    }

    // echo "bind-address = 127.0.0.1" >> /etc/mysql/mariadb.conf.d/50-server.cnf
    cmdStr := fmt.Sprintf(`echo "bind-address = 127.0.0.1" >> %s`, mariadbConf)
    if err := runCommand([]string{"sh", "-c", cmdStr}, ""); err != nil {
        return err
    }

    return nil
}

// Шаг 4. Генерируем 19-символьный логин и пароль (Go-методом)
func step4GenerateLoginAndPassword() (string, string, error) {
    log.Println("[Шаг 4] Генерирую 19-символьный логин и пароль для MySQL (Go-метод)...")

    login, err := generateRandomString(19)
    if err != nil {
        return "", "", fmt.Errorf("не удалось сгенерировать логин: %v", err)
    }
    password, err := generateRandomString(19)
    if err != nil {
        return "", "", fmt.Errorf("не удалось сгенерировать пароль: %v", err)
    }

    log.Printf("   Сгенерированный логин: %s\n", login)
    log.Printf("   Сгенерированный пароль: %s\n", password)

    return login, password, nil
}

// Шаг 5. Создаю MySQL-пользователя и даю ему права.
func step5CreateMySQLUser(login, password string) error {
    log.Println("[Шаг 5] Создаю MySQL-пользователя и даю ему права...")

    // sudo mysql -u root -e "CREATE USER '$login'@'localhost' IDENTIFIED BY '$password';
    //                        GRANT ALL ON *.* TO '$login'@'localhost';
    //                        FLUSH PRIVILEGES;"
    query := fmt.Sprintf(
        "CREATE USER '%s'@'localhost' IDENTIFIED BY '%s'; GRANT ALL ON *.* TO '%s'@'localhost'; FLUSH PRIVILEGES;",
        login, password, login,
    )
    return runCommand([]string{
        "sudo", "mysql", "-u", "root", "-e", query,
    }, "")
}

// Шаг 6. Создаю файл /root/auto_deploy/phpMyAdmin.txt с логином/паролем...
func step6SaveCredentials(login, password string) error {
    log.Println("[Шаг 6] Создаю файл", autoDeploy, "с логином/паролем...")

    // mkdir -p /root/auto_deploy
    if err := runCommand([]string{"mkdir", "-p", "/root/auto_deploy"}, ""); err != nil {
        return err
    }

    // echo "$login|$password" > /root/auto_deploy/phpMyAdmin.txt
    echoStr := fmt.Sprintf(`echo "%s|%s" > %s`, login, password, autoDeploy)
    return runCommand([]string{"sh", "-c", echoStr}, "")
}

// Шаг 7. Перезапускаю mariadb...
func step7RestartMariaDB() error {
    log.Println("[Шаг 7] Перезапускаю mariadb...")
    return runCommand([]string{"systemctl", "restart", "mariadb"}, "")
}

// Шаг 8. Проверяю конфигурацию Nginx...
func step8CheckNginx() error {
    log.Println("[Шаг 8] Проверяю конфигурацию Nginx (nginx -t)...")
    return runCommand([]string{"nginx", "-t"}, "")
}

// Шаг 9. Перезапускаю Nginx...
func step9RestartNginx() error {
    log.Println("[Шаг 9] Перезапускаю Nginx...")
    return runCommand([]string{"systemctl", "restart", "nginx"}, "")
}

func main() {
    // Оформим пошаговое выполнение с проверкой после каждого шага
    steps := []struct {
        name string
        fn   func() error
    }{
        {"[Шаг 1]", step1RemoveTestDBAndAnonymous},
        {"[Шаг 2]", step2RestartMariaDB},
        {"[Шаг 3]", step3SetBindAddress},
    }

    // Выполняем шаги 1-3
    for i, step := range steps {
        log.Println(step.name, "начало ...")
        if err := step.fn(); err != nil {
            log.Fatalf("[Ошибка на шаге %d]: %v", i+1, err)
        }
        log.Println(step.name, "выполнен успешно.")
    }

    // Шаг 4: Генерируем логин/пароль через Go-метод
    log.Println("[Шаг 4] начало ...")
    login, password, err := step4GenerateLoginAndPassword()
    if err != nil {
        log.Fatalf("[Ошибка на шаге 4]: %v", err)
    }
    log.Println("[Шаг 4] выполнен успешно.")

    // Шаги 5-9, которым нужны login и password
    nextSteps := []struct {
        name string
        fn   func() error
    }{
        {"[Шаг 5]", func() error { return step5CreateMySQLUser(login, password) }},
        {"[Шаг 6]", func() error { return step6SaveCredentials(login, password) }},
        {"[Шаг 7]", step7RestartMariaDB},
        {"[Шаг 8]", step8CheckNginx},
        {"[Шаг 9]", step9RestartNginx},
    }

    for j, step := range nextSteps {
        stepNumber := j + 5 // потому что 5..9
        log.Println(step.name, "начало ...")
        if err := step.fn(); err != nil {
            log.Fatalf("[Ошибка на шаге %d]: %v", stepNumber, err)
        }
        log.Println(step.name, "выполнен успешно.")
    }

    log.Println("Все шаги выполнены успешно!")
}
