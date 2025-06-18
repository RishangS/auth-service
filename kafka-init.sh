#!/bin/bash

KAFKA_CONTAINER=$(docker ps --filter ancestor=confluentinc/cp-kafka:7.4.0 --format "{{.ID}}")

if [ -z "$KAFKA_CONTAINER" ]; then
  echo "Kafka container not found!"
  exit 1
fi

docker exec -it "$KAFKA_CONTAINER" kafka-topics \
  --create \
  --topic chat-messages \
  --bootstrap-server localhost:9092 \
  --replication-factor 1 \
  --partitions 1
