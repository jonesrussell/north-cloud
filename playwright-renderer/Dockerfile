FROM mcr.microsoft.com/playwright:v1.58.2-jammy

WORKDIR /app

COPY package.json ./
RUN npm install --omit=dev

COPY server.js ./

ENV NODE_ENV=production
ENV RENDERER_PORT=8095

EXPOSE 8095

CMD ["node", "server.js"]
