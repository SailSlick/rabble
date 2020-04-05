from services.proto import general_pb2
from utils.articles import get_article, delete_article, get_sharers_of_article


class SendDeleteServicer:
    def __init__(self, logger, db, activ_util, users_util, hostname=None):
        self._logger = logger
        self._db = db
        self._activ_util = activ_util
        self._users_util = users_util
        self._hostname = hostname if hostname else self._activ_util._hostname

    def SendDeleteActivity(self, req, ctx):
        self._logger.info("Got request to delete article %d from %d",
                          req.article_id, req.user_id)
        user = self._users_util.get_user_from_db(global_id=req.user_id)
        if user is None:
            return general_pb2.GeneralResponse(
                result_type=general_pb2.ResultType.ERROR,
                error="Could not retrieve user",
            )
        article = get_article(self._logger, self._db, global_id=req.article_id)
        if article is None:
            return general_pb2.GeneralResponse(
                result_type=general_pb2.ResultType.ERROR,
                error="Could not retrieve article",
            )
        if article.author_id != req.user_id:
            self._logger.error("User requesting article deletion isn't author")
            return general_pb2.GeneralResponse(
                result_type=general_pb2.ResultType.ERROR_401,
                error="User is not the author of this article",
            )
        sharer_ids = get_sharers_of_article(
            self._logger, self._db, article.global_id)
        if not delete_article(self._logger, self._db, global_id=article.global_id):
            return general_pb2.GeneralResponse(
                result_type=general_pb2.ResultType.ERROR,
                error="Could not delete article locally",
            )
        delete_obj = self._activ_util.build_delete(
            user, article, self._hostname)
        self._logger.info("Activity: %s", str(delete_obj))
        err = self._activ_util.forward_activity_to_followers(
            req.user_id, delete_obj)
        if err is not None:
            return general_pb2.GeneralResponse(
                result_type=general_pb2.ResultType.ERROR,
                error=err,
            )
        # Send deletes of the article to the followers of people who
        # announced the article. There may be some duplicate Deletes
        # sent but this is acceptable.
        # This roughly the same pattern Mastodon follows:
        # https://github.com/tootsuite/mastodon/issues/5761#issuecomment-345875480
        for user_id in sharer_ids:
            err = self._activ_util.forward_activity_to_followers(
                user_id, delete_obj)
            if err is not None:
                # Warn but do not quit on error sending to announcer followers.
                self._logger.warning(
                    "Sending activity to followers of user %d failed", user_id)
        self._logger.info("Article %d successfully deleted", req.article_id)
        return general_pb2.GeneralResponse(
            result_type=general_pb2.ResultType.OK
        )
