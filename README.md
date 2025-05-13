# Web Page Analyzer - Test App

## Overview

- Go 1.23
- Frontend: Bootstrap

A web service that analyzes any given web page and extracts structured metadata, including HTML version, headings, links, and login forms. Designed with clean architecture and optimized for speed using Go concurrency.

### Key Features

- HTML Version Detection – Identify the document type (e.g., HTML5, XHTML).
- Title Extraction – Fetch the page title accurately.
- Heading Analysis – Count headings (h1-h6) and their distribution.
- Link Validation –
  - Categorize links as internal or external.
  - Detect inaccessible links (with count).
- Login Form Check – Determine if the page contains a login form.

### Technical Stack

**Frontend**

- HTML/CSS/JavaScript + AJAX for dynamic requests.
- Bootstrap for responsive UI.
- Hosted via Nginx.

**Backend**

- Go (Golang) with:
  - Goroutine for concurrent processing (reduced roundtrip time).- Clean Architecture + Adapter Pattern for maintainability.
  - Dependency Injection (Go-style).
- Dockerized for isolated deployment.

**Infrastructure**

- Docker containers for frontend/backend, connected via a Docker network.
- VS Code as the primary IDE.

Below URLs work after the deployment of the services according to the deployment section below.

- Web Page URL: ```http://localhost:8080/```
- Metrics URL: ```http://localhost:9090/metrics```
- Pprof URL: ```http://localhost:6060/debug/pprof/```

Backend API:

```shell
curl --location --request POST 'localhost:8090/analyze' \
--header 'x-request-id: 6c061f09-dc00-4cad-bf46-957cccf3f519' \
--header 'Content-Type: application/json' \
--data-raw '{
    "url": "https://medium.com/better-programming/awesome-logging-in-go-with-logrus-70606a49f2"
}'
```

### Project Structure

```MD
web_page_analyzer
├─ Dockerfile
├─ README.md
├─ docs
│  ├─ screencapture-localhost-8080-FE.png
│  └─ screencapture-localhost-9090-metrics.png
│  └─ screencapture-localhost-6060-debug-pprof.png
├─ go.mod
├─ go.sum
├─ internal
│  ├─ adaptors
│  │  ├─ web_client.go
│  │  └─ web_client_test.go
│  ├─ application
│  │  └─ config
│  │     └─ config.go
│  ├─ domain
│  │  ├─ adaptors
│  │  │  ├─ logger.go
│  │  │  └─ web_client.go
│  │  └─ models
│  │     └─ analysis_result.go
│  ├─ http
│  │  ├─ config.go
│  │  ├─ handlers
│  │  │  ├─ ready_handler.go
│  │  │  ├─ send_error.go
│  │  │  └─ web_page_analysis_handler.go
│  │  ├─ init.go
│  │  ├─ middleware
│  │  │  ├─ metrices.go
│  │  │  └─ request_id_logger.go
|  |  ├─ metrics_server.go
│  │  ├─ pprof_server.go
│  │  ├─ routes.go
│  │  └─ server.go
│  ├─ pkg
│  │  ├─ errors
│  │  │  ├─ error_test.go
│  │  │  └─ errors.go
│  │  └─ metrics
│  │     └─ metrics.go
│  └─ service
│     ├─ web_page_analyzer.go
│     └─ web_page_analyzer_test.go
├─ main.go
└─ web_page
   ├─ Dockerfile
   ├─ default.conf
   └─ index.html
```

### Prerequisites

- [Git](https://git-scm.com/downloads)
- [Go 1.23+](https://go.dev/doc/install)
- [Docker](https://docs.docker.com/desktop/setup/install/mac-install/)
- [VS Code](https://code.visualstudio.com/download)

### Setup the project

Execute below steps on VS Code terminal

- Install prerequisites.
- Clone or download repository as a zip file to your workspace folder. ex: $HOME/go/src

```shell
git clone git@github.com:Yahampath/web_page_analyzer.git
or 
git clone https://github.com/Yahampath/web_page_analyzer.git
```

- Download dependencies

```shell
go mod vendor
and 
go mod tidy
```

- Run backend

```shell
go run main.go
```

or

```shell
Go build
```

and then

```shell
./web_page_analyzer
```

## Dependencies

Below dependencies libraries use to develop and build and run this service

- github.com/go-chi/chi/v5 v5.2.1
- github.com/joho/godotenv v1.5.1
- github.com/sirupsen/logrus v1.9.3
- golang.org/x/sync v0.14.0

## Deployment

```shell
# Below command should executed in terminal from repository root folder.

docker build -t web-page-analyzer:v1.0.0 . # create a docker image for BE

docker container images # If image created, it should be showing in the results of this command.

docker create network web-page-analysis-network

docker network ls # check network created

docker run  -p 8090:8090 -p 9090:9090 -p 6060:6060 --network web-page-analysis-network --name web-page-analyzer-service web-page-analyzer:v1.0.0 # run docker image

cd web_page # go to the front-end root folder

docker build -t webpage-analyzer-web-ui:v1.0.0 . # create a docker image for FE

docker container images # If image created, it should be showing in the results of this command

docker run  --name web-page-analyzer-web-ui -p 8080:80 --network web-page-analysis-network webpage-analyzer-web-ui:v1.0.0
```

Open a browser and go to http://localhost:8080 for FE.

## Possible Improvement

- **Functional**

  - Improve go routing by implementing worker pool with context cancellation and paralyzes html doc analysis functionalities.
  - Improve logging by implementing proper logging format, logging with fields that relevant for flows.
  - Use DI container library for Dependency injection.
  - Improves errors by introducing flag to display error line information.

- **Non-Functional**

  - Create grafana dashboard for metrics and deploy prometheus and grafana servers in separate containers.
  - Forward logs into the logstash then kibana to improve log visibility.

## Screenshots

Front-end:
![Front-end](/docs/screencapture-localhost-8080-FE.png)

Metrics:
![metrics](/docs/screencapture-localhost-9090-metrics.png)

Pprof:
![pprof](/docs/screencapture-localhost-6060-debug-pprof.png)
