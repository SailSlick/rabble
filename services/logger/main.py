#!/usr/bin/env python3
from concurrent import futures
import argparse
import grpc
import logging
import os
import sys
import time

from logger.logger_servicer import LoggerServicer
from services.proto import logger_pb2_grpc

LOG_LEVEL_ENV_VAR = 'LOG_LEVEL'


def get_args():
    parser = argparse.ArgumentParser('Run the Rabble logger microservice')
    parser.add_argument('-f', default='rabble.log',
                        help='The file to write logs to')
    return parser.parse_args()


def get_local_logger(level):
    logger = logging.getLogger(__name__ + '_local')
    logger.addHandler(logging.StreamHandler())
    logger.setLevel(level)
    return logger


def get_file_logger(filename, level):
    logger = logging.getLogger(__name__)
    logger.addHandler(logging.FileHandler(filename))
    logger.setLevel(level)
    return logger


def main():
    log_level = os.environ.get(LOG_LEVEL_ENV_VAR)
    if not log_level:
        print("Exiting as {} env variable is not set".format(LOG_LEVEL_ENV_VAR))
        sys.exit(1)

    args = get_args()
    logger = get_file_logger(args.f, log_level)
    local_logger = get_local_logger(log_level)
    local_logger.info("Creating server")
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    logger_pb2_grpc.add_LoggerServicer_to_server(
        LoggerServicer(logger), server)
    server.add_insecure_port('0.0.0.0:1867')
    local_logger.info("Starting Logger service on port 1867")
    server.start()
    try:
        while True:
            time.sleep(60 * 60 * 24)  # One day
    except KeyboardInterrupt:
        pass


if __name__ == '__main__':
    main()
