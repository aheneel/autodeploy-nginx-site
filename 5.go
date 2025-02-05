package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "strings"
)

// runCommand выполняет указанную команду,
// выводя stdout и stderr напрямую в консоль.
// При ошибке возвращает error (что прерывает программу в main).
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

// Шаг 1. Создаю директорию /root/auto_deploy/templates (если не существует)...
func step1MakeTemplatesDir() error {
    log.Println("[Шаг 1] Создаю директорию /root/auto_deploy/templates (если не существует)...")
    return runCommand([]string{"mkdir", "-p", "/root/auto_deploy/templates"})
}

// Шаг 2. Создаю директорию /root/auto_deploy/log (если не существует)...
func step2MakeLogDir() error {
    log.Println("[Шаг 2] Создаю директорию /root/auto_deploy/log (если не существует)...")
    return runCommand([]string{"mkdir", "-p", "/root/auto_deploy/log"})
}

// Шаг 3. Создаю файлы deploy_wp.txt и cloudflare.txt (touch)
func step3TouchFiles() error {
    log.Println("[Шаг 3] Создаю (при необходимости) файлы deploy_wp.txt и cloudflare.txt...")
    // touch /root/auto_deploy/deploy_wp.txt
    // touch /root/auto_deploy/cloudflare.txt
    // В Go: просто открываем/создаём файлы на запись/чтение
    files := []string{"/root/auto_deploy/deploy_wp.txt", "/root/auto_deploy/cloudflare.txt"}
    for _, f := range files {
        _, err := os.OpenFile(f, os.O_CREATE|os.O_RDWR, 0644)
        if err != nil {
            return fmt.Errorf("не удалось создать или открыть файл '%s': %v", f, err)
        }
    }
    return nil
}

// Шаг 4. Создаю /root/auto_deploy/templates/nossl_nowww.conf.j2
func step4CreateNoSSLNoWWW() error {
    log.Println("[Шаг 4] Создаю и записываю /root/auto_deploy/templates/nossl_nowww.conf.j2...")

    content := `map $http_x_forwarded_proto $fastcgi_https {
    default off;
    https on;
}

# Определяем "плохих" ботов (регистронезависимо)
map $http_user_agent $is_bot {
    ~*(ahrefsbot|semrushbot|mj12bot|dotbot|lssbot|bingbot|yandexbot|mail\.ru_bot|spbot|scrapy|crawler|scanner) 1;
    default 0;
}

server {
    listen 80;
    server_name {{ domain_name }} www.{{ domain_name }};

    root /var/www/{{ domain_name }};
    index index.php index.html;

    # Безопасные заголовки (Security Headers)
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Permissions-Policy "accelerometer=(), camera=(), microphone=(), geolocation=(self)" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;

    # ОТКЛЮЧАЕМ кеширование на уровне HTTP-заголовков
    add_header Cache-Control "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0" always;
    add_header Pragma "no-cache" always;
    add_header Expires "0" always;

    # Если запрос пришёл по HTTP (X-Forwarded-Proto = http), перенаправляем сразу на HTTPS без www
    if ($http_x_forwarded_proto = 'http') {
        return 301 https://{{ domain_name }}$request_uri;
    }

    # Если запрос пришёл с www, перенаправляем на без www (уже https)
    if ($host = 'www.{{ domain_name }}') {
        return 301 https://{{ domain_name }}$request_uri;
    }

    location / {
        # Если бот, возвращаем 521
        if ($is_bot = 1) {
            return 521;
        }
        try_files $uri $uri/ /index.php?$args;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        fastcgi_param HTTPS $fastcgi_https;
    }

    location ~ /\.ht {
        deny all;
    }
}
`

    return os.WriteFile("/root/auto_deploy/templates/nossl_nowww.conf.j2", []byte(content), 0644)
}

// Шаг 5. Создаю /root/auto_deploy/templates/nossl_www.conf.j2
func step5CreateNoSSLWWW() error {
    log.Println("[Шаг 5] Создаю и записываю /root/auto_deploy/templates/nossl_www.conf.j2...")

    content := `map $http_x_forwarded_proto $fastcgi_https {
    default off;
    https on;
}

# Определяем "плохих" ботов
map $http_user_agent $is_bot {
    ~*(ahrefsbot|semrushbot|mj12bot|dotbot|lssbot|bingbot|yandexbot|mail\.ru_bot|spbot|scrapy|crawler|scanner) 1;
    default 0;
}

server {
    listen 80;
    server_name {{ domain_name }} www.{{ domain_name }};

    root /var/www/{{ domain_name }};
    index index.php index.html;

    # Безопасные заголовки (Security Headers)
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Permissions-Policy "accelerometer=(), camera=(), microphone=(), geolocation=(self)" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;

    # ОТКЛЮЧАЕМ кеширование на уровне HTTP-заголовков
    add_header Cache-Control "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0" always;
    add_header Pragma "no-cache" always;
    add_header Expires "0" always;

    # Если запрос пришёл по HTTP (X-Forwarded-Proto = http), перенаправляем сразу на HTTPS с www
    if ($http_x_forwarded_proto = 'http') {
        return 301 https://www.{{ domain_name }}$request_uri;
    }

    # Если запрос пришёл без www, перенаправляем на https://www
    if ($host = '{{ domain_name }}') {
        return 301 https://www.{{ domain_name }}$request_uri;
    }

    location / {
        # Если бот, возвращаем 521
        if ($is_bot = 1) {
            return 521;
        }
        try_files $uri $uri/ /index.php?$args;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        fastcgi_param HTTPS $fastcgi_https;
    }

    location ~ /\.ht {
        deny all;
    }
}
`
    return os.WriteFile("/root/auto_deploy/templates/nossl_www.conf.j2", []byte(content), 0644)
}

// Шаг 6. Создаю /root/auto_deploy/templates/ssl_nowww.conf.j2
func step6CreateSSLNoWWW() error {
    log.Println("[Шаг 6] Создаю и записываю /root/auto_deploy/templates/ssl_nowww.conf.j2...")

    content := `map $http_x_forwarded_proto $fastcgi_https {
    default off;
    https on;
}

map $http_user_agent $is_bot {
    ~*(ahrefsbot|semrushbot|mj12bot|dotbot|lssbot|bingbot|yandexbot|mail\.ru_bot|spbot|scrapy|crawler|scanner) 1;
    default 0;
}

# HTTP серверный блок
server {
    listen 80;
    server_name {{ domain_name }} www.{{ domain_name }};
    return 301 https://{{ domain_name }}$request_uri;
}

# HTTPS серверный блок
server {
    listen 443 ssl; 
    server_name {{ domain_name }} www.{{ domain_name }};

    ssl_certificate /etc/letsencrypt/live/{{ domain_name }}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/{{ domain_name }}/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf; 
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    root /var/www/{{ domain_name }};
    index index.php index.html;

    # Безопасные заголовки (Security Headers)
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Permissions-Policy "accelerometer=(), camera=(), microphone=(), geolocation=(self)" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;

    # ОТКЛЮЧАЕМ кеширование на уровне HTTP-заголовков
    add_header Cache-Control "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0" always;
    add_header Pragma "no-cache" always;
    add_header Expires "0" always;

    if ($host = 'www.{{ domain_name }}') {
        return 301 https://{{ domain_name }}$request_uri;
    }

    location / {
        # Если бот, возвращаем 521
        if ($is_bot = 1) {
            return 521;
        }
        try_files $uri $uri/ /index.php?$args;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        fastcgi_param HTTPS $fastcgi_https;
    }

    location ~ /\.ht {
        deny all;
    }
}
`
    return os.WriteFile("/root/auto_deploy/templates/ssl_nowww.conf.j2", []byte(content), 0644)
}

// Шаг 7. Создаю /root/auto_deploy/templates/ssl_www.conf.j2
func step7CreateSSLWWW() error {
    log.Println("[Шаг 7] Создаю и записываю /root/auto_deploy/templates/ssl_www.conf.j2...")

    content := `map $http_x_forwarded_proto $fastcgi_https {
    default off;
    https on;
}

map $http_user_agent $is_bot {
    ~*(ahrefsbot|semrushbot|mj12bot|dotbot|lssbot|bingbot|yandexbot|mail\.ru_bot|spbot|scrapy|crawler|scanner) 1;
    default 0;
}

# HTTP серверный блок
server {
    listen 80;
    server_name {{ domain_name }} www.{{ domain_name }};
    return 301 https://www.{{ domain_name }}$request_uri;
}

# HTTPS серверный блок
server {
    listen 443 ssl; 
    server_name {{ domain_name }} www.{{ domain_name }};

    ssl_certificate /etc/letsencrypt/live/{{ domain_name }}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/{{ domain_name }}/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf; 
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    root /var/www/{{ domain_name }};
    index index.php index.html;

    # Безопасные заголовки (Security Headers)
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Permissions-Policy "accelerometer=(), camera=(), microphone=(), geolocation=(self)" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;

    # ОТКЛЮЧАЕМ кеширование на уровне HTTP-заголовков
    add_header Cache-Control "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0" always;
    add_header Pragma "no-cache" always;
    add_header Expires "0" always;

    # Если пришли без www, то перенаправляем на www
    if ($host = '{{ domain_name }}') {
        return 301 https://www.{{ domain_name }}$request_uri;
    }

    location / {
        # Если бот, возвращаем 521
        if ($is_bot = 1) {
            return 521;
        }
        try_files $uri $uri/ /index.php?$args;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        fastcgi_param HTTPS $fastcgi_https;
    }

    location ~ /\.ht {
        deny all;
    }
}
`
    return os.WriteFile("/root/auto_deploy/templates/ssl_www.conf.j2", []byte(content), 0644)
}

func main() {
    // Весь список шагов: название + функция.
    steps := []struct {
        stepName string
        fn       func() error
    }{
        {"[Шаг 1]", step1MakeTemplatesDir},
        {"[Шаг 2]", step2MakeLogDir},
        {"[Шаг 3]", step3TouchFiles},
        {"[Шаг 4]", step4CreateNoSSLNoWWW},
        {"[Шаг 5]", step5CreateNoSSLWWW},
        {"[Шаг 6]", step6CreateSSLNoWWW},
        {"[Шаг 7]", step7CreateSSLWWW},
    }

    // Выполняем шаги по порядку
    for i, s := range steps {
        log.Printf("%s начало ...\n", s.stepName)
        if err := s.fn(); err != nil {
            log.Fatalf("[Ошибка на шаге %d]: %v", i+1, err)
        }
        log.Printf("%s выполнен успешно.\n", s.stepName)
    }

    log.Println("Все шаги выполнены успешно!")
}
