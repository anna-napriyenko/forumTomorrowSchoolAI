#!/bin/bash
# build.sh
docker image build -t forum:latest .
docker container run -d -p 8080:8080 --name forum -v $(pwd)/forum.db:/app/forum.db forum:latest
### chmod +x build.sh
### ./build.sh