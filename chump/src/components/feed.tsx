import * as React from "react";
import { Link } from "react-router-dom";

import { GetPublicPosts, IBlogPost } from "../models/posts";

interface IFeedState {
  publicBlog: IBlogPost[];
}

export class Feed extends React.Component<{}, IFeedState> {
  constructor(props: any) {
    super(props);
    this.state = { publicBlog: [] };
  }

  public componentDidMount() {
    this.getPosts();
  }

  public getPosts() {
    GetPublicPosts()
      .then((posts: IBlogPost[]) => {
        this.setState({ publicBlog: posts });
      })
      .catch(this.handleGetPostsErr);
  }

  public handleGetPostsErr() {
    alert("could not communicate with server :(");
  }

  public renderPosts() {
    return this.state.publicBlog.map((e: IBlogPost, i: number) => {
      // TODO(devoxel): Replace dangerouslySetInnerHTML with a safer option
      return (
        <div className="pure-g" key={i}>
          <div className="pure-u-1-5"/>
          <div className="pure-u-3-5">
            <h3 className="title">
              {e.title} <div className="byline"> by {e.author} </div>
            </h3>
            <p dangerouslySetInnerHTML={{ __html: e.body }}/>
          </div>
        </div>
      );
    });
  }

  public render() {
    const blogPosts = this.renderPosts();
    return (
      <div>
        <div className="pure-g">
          <div className="pure-u-1-5"/>
          <div className="pure-u-3-5">
            <h3>Your blog post could be here!</h3>
            <p>Check out our <Link to="/about">about</Link> page for more info!</p>
          </div>
        </div>
        {blogPosts}
      </div>
    );
  }
}