FROM nginx:1.25-alpine

RUN apk add --no-cache curl

# Config is also mounted at runtime via compose volume (./nginx/conf.d),
# this COPY only provides a safe default if no volume is mounted.
COPY nginx.conf /etc/nginx/nginx.conf
COPY conf.d /etc/nginx/conf.d

EXPOSE 80 443
CMD ["nginx", "-g", "daemon off;"]
