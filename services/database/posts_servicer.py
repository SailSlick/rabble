import sqlite3

import util

from services.proto import database_pb2
from services.proto import database_pb2_grpc
from google.protobuf.timestamp_pb2 import Timestamp


class PostsDatabaseServicer:

    def __init__(self, db, logger):
        self._db = db
        self._logger = logger
        self._type_handlers = {
            database_pb2.PostsRequest.INSERT: self._handle_insert,
            database_pb2.PostsRequest.FIND: self._handle_find,
            database_pb2.PostsRequest.DELETE: self._handle_delete,
            database_pb2.PostsRequest.UPDATE: self._handle_update,
        }

    def Posts(self, request, context):
        response = database_pb2.PostsResponse()
        self._type_handlers[request.request_type](request, response)
        return response

    def _handle_insert(self, req, resp):
        try:
            self._db.execute(
                'INSERT INTO posts '
                '(author_id, title, body, creation_datetime, md_body, ap_id) '
                'VALUES (?, ?, ?, ?, ?, ?)',
                req.entry.author_id, req.entry.title,
                req.entry.body,
                req.entry.creation_datetime.seconds,
                req.entry.md_body,
                req.entry.ap_id,
                commit=False)
            res = self._db.execute(
                'SELECT last_insert_rowid() FROM posts LIMIT 1')
        except sqlite3.Error as e:
            resp.result_type = database_pb2.PostsResponse.ERROR
            resp.error = str(e)
            return
        if len(res) != 1 or len(res[0]) != 1:
            err = "Global ID data in weird format: " + str(res)
            self._logger.error(err)
            resp.result_type = database_pb2.PostsResponse.ERROR
            resp.error = err
            return
        resp.result_type = database_pb2.PostsResponse.OK
        resp.global_id = res[0][0]

    def _db_tuple_to_entry(self, tup, entry):
        if len(tup) != 7:
            self._logger.warning(
                "Error converting tuple to PostsEntry: " +
                "Wrong number of elements " + str(tup))
            return False
        try:
            # You'd think there'd be a better way.
            entry.global_id = tup[0]
            entry.author_id = tup[1]
            entry.title = tup[2]
            entry.body = tup[3]
            entry.creation_datetime.seconds = tup[4]
            entry.md_body = tup[5]
            entry.ap_id = tup[6]
        except Exception as e:
            self._logger.warning(
                "Error converting tuple to PostsEntry: " +
                str(e))
            return False
        return True

    def _handle_find(self, req, resp):
        filter_clause, values = util.entry_to_filter(req.match)
        try:
            if not filter_clause:
                res = self._db.execute('SELECT * FROM posts')
            else:
                res = self._db.execute(
                    'SELECT * FROM posts WHERE ' + filter_clause,
                    *values)
        except sqlite3.Error as e:
            resp.result_type = database_pb2.PostsResponse.ERROR
            resp.error = str(e)
            return
        resp.result_type = database_pb2.PostsResponse.OK
        for tup in res:
            if not self._db_tuple_to_entry(tup, resp.results.add()):
                del resp.results[-1]

    def _handle_delete(self, req, resp):
        filter_clause, values = util.entry_to_filter(req.match)
        try:
            if not filter_clause:
                res = self._db.execute('DELETE FROM posts')
            else:
                res = self._db.execute(
                    'DELETE FROM posts WHERE ' + filter_clause,
                    *values)
        except sqlite3.Error as e:
            resp.result_type = database_pb2.PostsResponse.ERROR
            resp.error = str(e)
            return
        resp.result_type = database_pb2.PostsResponse.OK

    def _handle_update(self, req, resp):
        pass
