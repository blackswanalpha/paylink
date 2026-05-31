# mpesa-daraja rail service (work04) — Node.js. Self-contained: build context is THIS directory
# (no cross-module deps, unlike the Go core). No runtime npm dependencies (built-ins only), so no
# install step is needed.
FROM node:20-alpine
WORKDIR /app
COPY package.json ./
COPY src/ ./src/
EXPOSE 8083
USER node
CMD ["node", "src/index.js"]
