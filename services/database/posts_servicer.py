import sqlite3

from users_servicer import UsersDatabaseServicer

import database_pb2
import database_pb2_grpc
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
                '(global_id, author, title, body, creation_datetime) '
                'VALUES (?, ?, ?, ?, ?)',
                req.entry.global_id, req.entry.author,
                req.entry.title, req.entry.body,
                req.entry.creation_datetime.seconds)
        except sqlite3.Error as e:
            resp.result_type = database_pb2.PostsResponse.ERROR
            resp.error = str(e)
            return
        resp.result_type = database_pb2.PostsResponse.OK

    def _db_tuple_to_entry(self, tup, entry):
        if len(tup) != 5:
            self._logger.warning(
                "Error converting tuple to PostsEntry: " +
                "Wrong number of elements " + str(tup))
            return False
        try:
            # You'd think there'd be a better way.
            entry.global_id = tup[0]
            entry.author = tup[1]
            entry.title = tup[2]
            entry.body = tup[3]
            entry.creation_datetime.seconds = tup[4]
        except Exception as e:
            self._logger.warning(
                "Error converting tuple to PostsEntry: " +
                str(e))
            return False
        return True

    def _entry_to_filter(self, entry):
        fields = entry.ListFields()
        filter_list = [f.name + ' = ?' for f, _ in fields]
        filter_clause = ' AND '.join(filter_list)
        return filter_clause, [v for _, v in fields]

    def _handle_find(self, req, resp):
        filter_clause, values = self._entry_to_filter(req.match)
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
        filter_clause, values = self._entry_to_filter(req.match)
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
