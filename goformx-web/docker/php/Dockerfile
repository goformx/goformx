FROM php:8.4-fpm-alpine

RUN apk add --no-cache nginx supervisor curl

# PHP extensions
RUN docker-php-ext-install pdo pdo_mysql opcache

# Composer
COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /app

# Nginx config
COPY nginx.conf /etc/nginx/http.d/default.conf

# Supervisor config
RUN printf "[supervisord]\nnodaemon=true\n\n[program:php-fpm]\ncommand=php-fpm\n\n[program:nginx]\ncommand=nginx -g 'daemon off;'\n" > /etc/supervisor/conf.d/supervisord.conf

EXPOSE 80

CMD ["supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
