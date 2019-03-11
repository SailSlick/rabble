from services.proto import s2s_follow_pb2

class SendFollowServicer:

    def __init__(self, logger, activ_util):
        self._logger = logger
        self._activ_util = activ_util

    def _build_activity(self, follower_actor, followed_actor, sendable=True):
        '''Build a follow activity for actor `follower_actor` following
        actor `followed_actor`. If `sendable` is set to True, this function
        will add the extra fields to the JSON to turn it into a proper fully
        qualified and ready-to-send Activity (ie. the context, `to` field, etc).
        If `sendable` is not True, then it will generate an Activity that can
        be embedded in another one (ie. an Undo).'''
        d = {
            'type': 'Follow',
            'actor': follower_actor,
            'object': followed_actor,
        }
        if sendable:
            d['@context'] = self._activ_util.rabble_context()
            d['to'] = [followed_actor]
        return d


    def _build_delete(self, deleter_actor, follow_activity):
        d = {
            '@context':  self._activ_util.rabble_context(),
            'type': 'Undo',
            'actor': deleter_actor,
            'object': follow_activity,
            'to': []
        }

    def SendFollowActivity(self, req, context):
        resp = s2s_follow_pb2.FollowActivityResponse()
        follower_actor = self._activ_util.build_actor(
            req.follower.handle, req.follower.host)
        followed_actor = self._activ_util.build_actor(
            req.followed.handle, req.followed.host)
        activity = self._build_activity(follower_actor, followed_actor)
        inbox_url = self._activ_util.build_inbox_url(
            req.followed.handle, req.followed.host)
        self._logger.debug('Sending follow activity to foreign server')
        _, err = self._activ_util.send_activity(activity, inbox_url)
        # TODO(iandioch): See if response was what was expected.
        if err is None:
            resp.result_type = s2s_follow_pb2.FollowActivityResponse.OK
        else:
            resp.result_type = s2s_follow_pb2.FollowActivityResponse.ERROR
            resp.error = err
        return resp

    def SendUnfollowActivity(self, req, context):
        resp = s2s_follow_pb2.FollowActivityResponse()

        follower_actor = self._activ_util.build_actor(
            req.follower.handle, req.follower.host)
        followed_actor = self._activ_util.build_actor(
            req.followed.handle, req.followed.host)
        # Build a follow activity, then wrap it in an Undo activity
        follow_activity = self._build_activity(follower_actor,
                                               followed_actor,
                                               sendable=False)
        delete_activity = self._build_delete(follower_actor, follow_activity)

        inbox_url = self._activ_util.build_inbox_url(
            req.followed.handle, req.followed.host)

        self._logger.debug('Sending unfollow activity to foreign server')
        _, err = self._activ_util.send_activity(activity, inbox_url)
        # TODO(iandioch): See if response was what was expected.
        if err is None:
            resp.result_type = s2s_follow_pb2.FollowActivityResponse.OK
        else:
            resp.result_type = s2s_follow_pb2.FollowActivityResponse.ERROR
            resp.error = err


        return resp
