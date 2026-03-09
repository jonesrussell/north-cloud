FROM mcr.microsoft.com/playwright:v1.52.0-noble

WORKDIR /app
COPY package.json ./
RUN npm install --production
COPY handler.js stealth.js index.js ./

EXPOSE 3000
CMD ["node", "index.js"]
