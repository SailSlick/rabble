import * as React from "react";
import { Trash2 } from "react-feather";
import * as RModal from "react-modal";
import * as request from "superagent";

import * as config from "../../../rabble_config.json";
import { DeleteArticle } from "../../models/article";
import { IParsedPost } from "../../models/posts";
import { RootComponent } from "../root_component";

interface IDeleteProps {
  successCallback: () => void;
  username: string;
  display: boolean;
  blogPost: IParsedPost;
}

export class DeleteButton extends RootComponent<IDeleteProps, {}> {
  constructor(props: IDeleteProps) {
    super(props);
    this.handleDelete = this.handleDelete.bind(this);
  }

  public render() {
    if (!this.props.display) {
      return null;
    }
    return (
      <div className="pure-u-5-24">
        <button
          className="pure-button pure-input-1-3 pure-button-primary primary-button"
          onClick={this.handleDelete}
        >
          <Trash2/> Delete
        </button>
      </div>
    );
  }

  private handleDelete() {
    if (!window.confirm(config.delete_confirm)) {
      return;
    }
    DeleteArticle(this.props.blogPost.global_id)
      .then((res: request.Response) => {
        if (res.status < 200 || res.status >= 300) {
          this.errorToast({ statusCode: res.status, debug: res });
          return;
        }
        this.successToast("Article deleted successfully");
        this.props.successCallback();
      })
      .catch((err: Error) => {
        this.errorToast({ debug: err });
      });
  }
}
