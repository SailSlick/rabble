#!/usr/bin/env python3
from concurrent import futures
import grpc
import time

from utils.logger import get_logger
from ldnormaliser.servicer import LDNormServicer
from services.proto import ldnorm_pb2_grpc


def main():
    logger = get_logger("ldnorm_service")
    logger.info("Creating ldnorm server")
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    ldnorm_pb2_grpc.add_LDNormServicer_to_server(
        LDNormServicer(logger), server)
    server.add_insecure_port('0.0.0.0:1804')
    logger.info("Starting ldnorm server on port 1804")
    server.start()
    try:
        while True:
            time.sleep(60 * 60 * 24)  # One day
    except KeyboardInterrupt:
        pass


if __name__ == '__main__':
    main()
