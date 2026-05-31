# LinkMint api-gateway — Kong 3.7 (DB-less). Renders the declarative config from env at start.
FROM kong:3.7

# envsubst (gettext-base) is used by the entrypoint to render kong.yml.tmpl -> kong.yml.
USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends gettext-base \
    && rm -rf /var/lib/apt/lists/*

COPY kong/ /kong/
COPY docker-entrypoint.sh /usr/local/bin/lm-entrypoint.sh
RUN chmod +x /usr/local/bin/lm-entrypoint.sh && chown -R kong:kong /kong

# Defaults (override via env). Admin API bound to localhost only — never exposed.
ENV KONG_DATABASE=off \
    KONG_DECLARATIVE_CONFIG=/kong/kong.yml \
    KONG_PROXY_LISTEN=0.0.0.0:8088 \
    KONG_ADMIN_LISTEN=127.0.0.1:8001 \
    KONG_STATUS_LISTEN=0.0.0.0:8100 \
    KONG_VAULTS=env \
    KONG_PLUGINS=bundled \
    KONG_NGINX_WORKER_PROCESSES=1 \
    KONG_UNTRUSTED_LUA=sandbox \
    KONG_UNTRUSTED_LUA_SANDBOX_REQUIRES=cjson,cjson.safe

USER kong

# 8088 = proxy (the single ingress); 8100 = status/metrics.
EXPOSE 8088 8100

HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=10 \
    CMD kong health

ENTRYPOINT ["/usr/local/bin/lm-entrypoint.sh"]
CMD ["kong", "docker-start"]
