FROM alpine:3.22

COPY .env.production.example /bundle/.env.production.example
COPY deploy/docker-compose.prod.yml /bundle/deploy/docker-compose.prod.yml
COPY deploy/mongo/init/001_indexes.js /bundle/deploy/mongo/init/001_indexes.js
COPY deploy/mongo/prod-init/002_app_user.js /bundle/deploy/mongo/prod-init/002_app_user.js
COPY deploy/infra/ /bundle/deploy/infra/
COPY docs/production-deployment.md /bundle/README.md
COPY scripts/install-prod.sh /bundle/install-prod.sh
COPY scripts/render-nginx-prod.sh /bundle/render-nginx-prod.sh
COPY CHANGELOG.md /bundle/CHANGELOG.md

CMD ["true"]
