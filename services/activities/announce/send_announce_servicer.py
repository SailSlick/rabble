from services.proto import database_pb2
from services.proto import general_pb2
from activities.announce.announce_util import AnnounceUtil


class SendAnnounceServicer:
    def __init__(self, logger, db, users_util, activ_util, hostname=None):
        self._logger = logger
        self._db = db
        self._users_util = users_util
        self._activ_util = activ_util
        # Use the hostname passed in or get it manually
        self._hostname = hostname if hostname else self._activ_util._hostname
        self._announce_util = AnnounceUtil(
            logger, db, self._activ_util, self._hostname)

    def _get_shared_article(self, article_id):
        posts_req = database_pb2.PostsRequest(
            request_type=database_pb2.RequestType.FIND,
            match=database_pb2.PostsEntry(
                global_id=article_id,
            ),
        )
        resp = self._db.Posts(posts_req)
        if resp.result_type != general_pb2.ResultType.OK:
            return None, resp.error
        elif len(resp.results) > 1:
            return None, "Recieved too many results from DB"
        elif len(resp.results) == 0:
            return None, "No matching DB entry for this article"
        return resp.results[0], None

    def SendAnnounceActivity(self, req, context):
        self._logger.debug("Sending announce for article_id %s from %s at %s",
                           req.article_id,
                           req.announcer_id,
                           req.announce_time.seconds)
        response = general_pb2.GeneralResponse(
            result_type=general_pb2.ResultType.OK)

        announcer = self._users_util.get_user_from_db(
            global_id=req.announcer_id)
        if announcer is None:
            response.result_type = general_pb2.ResultType.ERROR_400
            response.error = "Announcer does not exist"
            return response

        # Get post author & reblogged article
        article, err = self._get_shared_article(req.article_id)
        if err is not None:
            response.result_type = general_pb2.ResultType.ERROR_400
            response.error = "Shared Article does not exist"
            return response

        author = self._users_util.get_user_from_db(global_id=article.author_id)
        if author is None:
            response.result_type = general_pb2.ResultType.ERROR_400
            response.error = "Author of shared post does not exist"
            return response
        if author.global_id == announcer.global_id:
            response.result_type = general_pb2.ResultType.ERROR_400
            response.error = "Author cannot share their own post"
            return response

        host = author.host
        article_url = None
        # check if author is local, if they are build local article url
        if not author.host:
            host = self._hostname
            article_url = self._activ_util.build_local_article_url(
                author, article)
        author_actor = self._activ_util.build_actor(author.handle, host)

        # Create Announce activity
        article_ap_id = self._activ_util.build_article_ap_id(author, article)
        timestamp = self._activ_util.timestamp_to_rfc(
            article.creation_datetime)
        article_obj = self._activ_util.build_article(
            article_ap_id, article.title, timestamp,
            author_actor, article.md_body, article.summary,
            article_url=article_url,
        )
        actor = self._activ_util.build_actor(announcer.handle, self._hostname)
        creation_datetime = self._activ_util.timestamp_to_rfc(
            req.announce_time)

        announce_activity = self._announce_util.build_announce_activity(
            actor, article_obj, creation_datetime)

        # Create a list of foreign followers
        follow_list = self._users_util.get_follower_list(req.announcer_id)
        foreign_follows = self._users_util.remove_local_users(follow_list)

        # Add article author if not in list
        # This is so they can increment their share count & supply to followers
        if author not in foreign_follows:
            foreign_follows.append(author)

        # Add announcer so local instance can add to share db
        if announcer not in foreign_follows:
            foreign_follows.append(announcer)

        # Send activity to all followers
        response = self._announce_util.send_announce_activity(
            foreign_follows, announce_activity, response)

        return response
