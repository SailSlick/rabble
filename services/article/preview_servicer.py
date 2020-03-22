from services.proto import article_pb2
from services.proto import general_pb2
from utils.articles import md_to_html


class PreviewServicer:

    def __init__(self, md_stub, logger):
        self._md_stub = md_stub
        self._logger = logger

    def PreviewArticle(self, req, context):
        self._logger.info('Recieved a new article to Preview.')
        html_body = md_to_html(self._md_stub, req.body)
        na = article_pb2.NewArticle(
            author_id=req.author_id,
            title=req.title,
            body=html_body,
            creation_datetime=req.creation_datetime
        )
        resp = article_pb2.PreviewResponse(
            preview=na,
            result_type=general_pb2.ResultType.OK
        )
        return resp
