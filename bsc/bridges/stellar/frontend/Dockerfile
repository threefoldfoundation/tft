FROM node:14 as build-stage
WORKDIR /app
COPY package*.json ./
COPY yarn.lock ./
RUN yarn install
COPY ./ .
RUN yarn build

FROM nginx as production-stage
RUN mkdir /app
COPY --from=build-stage /app/out /app
COPY nginx.conf /etc/nginx/nginx.conf
    
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]