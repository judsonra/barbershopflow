# syntax=docker/dockerfile:1.7
FROM node:22-alpine AS dependencies
WORKDIR /app
COPY frontend/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm install

FROM dependencies AS build
COPY frontend/ .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
