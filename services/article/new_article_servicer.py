from services.proto import article_pb2
from services.proto import database_pb2
from services.proto import create_pb2
from services.proto import search_pb2
from services.proto import general_pb2
from utils.articles import convert_to_tags_string, md_to_html


class NewArticleServicer:

    def __init__(self, create_stub, db_stub, md_stub, search_stub, logger, users_util, post_recommendation_stub=None):
        self._create_stub = create_stub
        self._db_stub = db_stub
        self._md_stub = md_stub
        self._search_stub = search_stub
        self._logger = logger
        self._users_util = users_util
        self._post_recommendation_stub = post_recommendation_stub

    def index(self, post_entry):
        """
        index takes a post proto and indexes it in the search service/

        Arguments:
        - post_entry (database.PostsEntry): A proto representing the post.
          This should have a valid global_id field.
        """
        req = search_pb2.IndexRequest(post=post_entry)
        resp = self._search_stub.Index(req)

        if resp.error:
            self._logger.warning("Error indexing post: %s", resp.error)

        return resp.result_type == general_pb2.ResultType.OK

    def send_insert_request(self, req):
        global_id = req.author_id
        author = self._users_util.get_user_from_db(global_id=global_id)
        if author is None:
            self._logger.error(
                'Could not find user id in db: ' + str(global_id))
            return database_pb2.PostsResponse.error, None
        global_id = author.global_id

        html_body = md_to_html(self._md_stub, req.body)
        tags_string = convert_to_tags_string(req.tags)
        pe = database_pb2.PostsEntry(
            author_id=global_id,
            title=req.title,
            body=html_body,
            md_body=req.body,
            creation_datetime=req.creation_datetime,
            ap_id=req.ap_id,
            tags=tags_string,
            summary=req.summary,
        )
        pr = database_pb2.PostsRequest(
            request_type=database_pb2.RequestType.INSERT,
            entry=pe
        )
        posts_resp = self._db_stub.Posts(pr)
        if posts_resp.result_type == general_pb2.ResultType.ERROR:
            self._logger.error(
                'Could not insert into db: %s', posts_resp.error)

        pe.global_id = posts_resp.global_id
        self.index(pe)

        # If post_recommender is on, send new post to post_recommender
        if self._post_recommendation_stub is not None:
            self._add_post_to_recommender(pe)

        return posts_resp.result_type, posts_resp.global_id

    def send_create_activity_request(self, req, global_id):
        html_body = md_to_html(self._md_stub, req.body)
        ad = create_pb2.ArticleDetails(
            author_id=req.author_id,
            title=req.title,
            body=html_body,
            md_body=req.body,
            creation_datetime=req.creation_datetime,
            global_id=global_id,
            summary=req.summary,
        )
        create_resp = self._create_stub.SendCreate(ad)

        return create_resp.result_type

    def _add_post_to_recommender(self, post_entry):
        resp = self._post_recommendation_stub.AddPost(post_entry)
        if resp.result_type != general_pb2.ResultType.OK:
            self._logger.error(
                "AddPost for post recommendation failed: %s", resp.message)

    def CreateNewArticle(self, req, context):
        self._logger.info('Recieved a new article.')
        success, global_id = self.send_insert_request(req)

        resp = article_pb2.NewArticleResponse()
        if success == general_pb2.ResultType.OK:
            self._logger.info('Article created.')
            resp.result_type = general_pb2.ResultType.OK
            resp.global_id = str(global_id)
            if not req.foreign:
                # TODO (sailslick) persist create activities
                # or add to queueing service
                create_success = self.send_create_activity_request(
                    req, global_id)
                if create_success == general_pb2.ResultType.ERROR:
                    self._logger.error('Could not send create Activity')
        else:
            resp.result_type = general_pb2.ResultType.ERROR
        return resp
