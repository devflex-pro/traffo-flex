FROM node:20-alpine
WORKDIR /app
COPY apps/admin-frontend/package.json ./
RUN npm install
COPY apps/admin-frontend ./
EXPOSE 5173
CMD ["npm", "run", "dev"]
