# InfluxDB v1

This example shows how to use the k6 performance test with InfluxDB v1.

## Prerequisites

- Docker
- Docker Compose

## Run the example

```bash
docker-compose up -d
```

## Access the k6 performance test dashboard

Open the k6 performance test dashboard in your browser http://localhost:3000/d/Le2Ku9NMk/k6-performance-test

## Run the k6 test

The test will run for 20 iterations and use 5 virtual users.

```bash
k6 -i 20 --vus 5 --out influxdb=http://localhost:8086/k6 run script.js
```
