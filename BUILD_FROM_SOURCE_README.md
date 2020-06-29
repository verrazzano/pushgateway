# Build Instructions

The base tag this release is branched from is `v1.2.0`


Create Environment Variables

```
export DOCKER_REPO=<Docker Repository>
export DOCKER_NAMESPACE=<Docker Namespace>
export DOCKER_TAG=<Image Tag>
```

Build and Push Images

```
docker build --file Dockerfile_verrazzano --tag ${DOCKER_REPO}/${DOCKER_NAMESPACE}/pushgateway-mirror:${DOCKER_TAG} .
docker push ${DOCKER_REPO}/${DOCKER_NAMESPACE}/pushgateway-mirror:${DOCKER_TAG}
``