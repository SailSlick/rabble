import * as React from "react";

import { GetPublicPosts } from "../models/posts";
import { FeedBody } from "./feed_body";
import { RootComponent } from "./root_component";

import * as config from "../../rabble_config.json";

interface IFeedProps {
  username: string;
  queryUserId: number;
}

export class Feed extends RootComponent<IFeedProps, {}> {
  constructor(props: IFeedProps) {
    super(props);
    this.state = {};
  }

  public render() {
    let feedHeader = config.feed_title;
    if (this.props.queryUserId !== 0) {
      feedHeader = config.user_feed_title;
    }
    return (
      <FeedBody
        username={this.props.username}
        queryUserId={this.props.queryUserId}
        feedTitle={feedHeader}
        GetPosts={GetPublicPosts}
      />
    );
  }
}
