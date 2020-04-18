from services.proto import database_pb2 as dbpb
from services.proto import update_pb2 as upb
from services.proto import general_pb2
from utils.articles import md_to_html


class ReceiveUpdateServicer:
    def __init__(self, logger, db, md, activ_util, users_util, hostname=None):
        self._logger = logger
        self._db = db
        self._md = md
        self._activ_util = activ_util
        self._users_util = users_util
        self._hostname = hostname if hostname else self._activ_util._hostname

    def ReceiveUpdateActivity(self, req, ctx):
        self._logger.info("Received edit for article '%s'", req.title)
        html_body = md_to_html(self._md, req.body)
        resp = self._db.Posts(dbpb.PostsRequest(
            request_type=dbpb.RequestType.UPDATE,
            match=dbpb.PostsEntry(ap_id=req.ap_id),
            entry=dbpb.PostsEntry(
                title=req.title,
                body=html_body,
                md_body=req.body,
                summary=req.summary,
            ),
        ))
        if resp.result_type != general_pb2.ResultType.OK:
            self._logger.error("Could not update article: %s", resp.error)
            return general_pb2.GeneralResponse(
                result_type=upb.UpdateRespones.ERROR,
                error="Error updating article in DB",
            )
        return general_pb2.GeneralResponse(
            result_type=general_pb2.ResultType.OK
        )
