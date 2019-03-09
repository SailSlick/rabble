import sqlite3

from services.proto import database_pb2 as db_pb


DEFAULT_NUM_POSTS = 25


class ShareDatabaseServicer:
    def __init__(self, db, logger):
        self._db = db
        self._logger = logger
        self._select_base = (
            "SELECT "
            "p.global_id, p.author_id, p.title, p.body, "
            "p.creation_datetime, p.md_body, p.ap_id, p.likes_count, "
            "l.user_id IS NOT NULL, f.follower IS NOT NULL, "
            "s.user_id IS NOT NULL "
            "FROM posts p LEFT OUTER JOIN likes l ON "
            "l.article_id=p.global_id AND l.user_id=? "
            "LEFT OUTER JOIN shares s ON "
            "s.article_id=p.global_id AND s.user_id=? "
            "LEFT OUTER JOIN follows f ON "
            "f.followed=p.author_id AND f.follower=? "
        )

    def SharedPosts(self, request, context):
        resp = database_pb2.PostsResponse()
        n = request.num_posts
        if not n:
            n = DEFAULT_NUM_POSTS
        user_id = -1
        if request.HasField("user_global_id"):
            user_id = request.user_global_id.value
        self._logger.info('Reading {} shared posts for user feed'.format(n))
        try:
            res = self._db.execute(self._select_base +
                                   'INNER JOIN shares s '
                                   'ON p.global_id = s.article_id AND s.user_id = ? '
                                   'ORDER BY p.global_id DESC '
                                   'LIMIT ?', user_id, user_id, user_id, user_id, n)
            for tup in res:
                if not self._db_tuple_to_entry(tup, resp.results.add()):
                    del resp.results[-1]
        except sqlite3.Error as e:
            resp.result_type = database_pb2.PostsResponse.ERROR
            resp.error = str(e)
            return resp
        return resp

    def _db_tuple_to_entry(self, tup, entry):
        if len(tup) != 13:
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
            entry.likes_count = tup[7]
            entry.is_liked = tup[8]
            entry.is_followed = tup[9]
            entry.is_shared = tup[10]
            entry.announce_datetime.seconds = tup[11]
            entry.sharer = tup[12]
        except Exception as e:
            self._logger.warning(
                "Error converting tuple to PostsEntry: " +
                str(e))
            return False
        return True

    def AddShare(self, req, context):
        self._logger.debug(
            "Adding share by %d to article %d",
            req.user_id, req.article_id
        )
        response = db_pb.AddShareResponse(
            result_type=db_pb.AddShareResponse.OK
        )
        try:
            self._db.execute(
                'INSERT INTO shares (user_id, article_id, announce_datetime) '
                'VALUES (?, ?)',
                req.user_id,
                req.article_id,
                req.announce_datetime,
                commit=False
            )
            self._db.execute(
                'UPDATE posts SET shares_count = shares_count + 1 '
                'WHERE global_id=?',
                req.article_id
            )
        except sqlite3.Error as e:
            self._db.commit()
            self._logger.error("AddLike error: %s", str(e))
            response.result_type = db_pb.AddLikeResponse.ERROR
            response.error = str(e)
        return response
