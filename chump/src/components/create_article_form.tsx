import * as React from "react";
import * as RModal from "react-modal";
import { HashRouter } from "react-router-dom";
import { CreateArticle, CreatePreview } from "../models/article";
import { IBlogPost } from "../models/posts";
import { Post } from "./post";

interface IFormState {
  blogText: string;
  post: IBlogPost;
  showModal: boolean;
  title: string;
}

export interface IFormProps {
  username: string;
}

export class CreateArticleForm extends React.Component<IFormProps, IFormState> {
  constructor(props: IFormProps) {
    super(props);

    this.state = {
      blogText: "",
      post: {
        author: "string",
        body: "string",
        global_id: 3,
        title: "string",
        likes_count: 0,
      },
      showModal: false,
      title: "",
    };

    this.handleTitleInputChange = this.handleTitleInputChange.bind(this);
    this.handleTextAreaChange = this.handleTextAreaChange.bind(this);
    this.handleSubmitForm = this.handleSubmitForm.bind(this);
    this.handlePreview = this.handlePreview.bind(this);
    this.handleClosePreview = this.handleClosePreview.bind(this);
    this.alertUser = this.alertUser.bind(this);
    this.renderModal = this.renderModal.bind(this);
  }

  public renderModal() {
    return (
      <div>
        <RModal
           isOpen={this.state.showModal}
           ariaHideApp={false}
        >
          <div className="pure-g topnav">
            <div className="pure-u-10-24">
              <button
                className="pure-button pure-input-1-3 pure-button-primary"
                onClick={this.handleClosePreview}
              >
                Close Preview
              </button>
            </div>
          </div>
          <div className="pure-g" key={1}>
            <HashRouter>
            <Post blogPost={this.state.post}/>
            </HashRouter>
          </div>
        </RModal>
      </div>
    );
  }

  public render() {
    const previewModel = this.renderModal();
    return (
      <div>
        <form
          className="pure-form pure-form-aligned"
          onSubmit={this.handleSubmitForm}
          id="create_post_form"
        >
          <div className="pure-control-group">
            <input
              type="text"
              name="title"
              value={this.state.title}
              onChange={this.handleTitleInputChange}
              className="pure-input-1-2"
              placeholder="Title"
            />
            <textarea
              name="blogText"
              value={this.state.blogText}
              onChange={this.handleTextAreaChange}
              className="pure-input-1 blog-input"
              placeholder="Start here"
            />
          </div>
        </form>
        <div className="pure-button-group" role="group">
          <button
            onClick={this.handlePreview}
            className="pure-button pure-input-1-3 pure-button-primary"
          >
            Preview
          </button>
          <button
            type="submit"
            className="pure-button pure-input-1-3 pure-button-primary"
            form="create_post_form"
          >
            Post
          </button>
        </div>
        {previewModel}
      </div>
    );
  }

  private handleTitleInputChange(event: React.ChangeEvent<HTMLInputElement>) {
    const target = event.target;
    this.setState({
      title: target.value,
    });
  }

  private handleTextAreaChange(event: React.ChangeEvent<HTMLTextAreaElement>) {
    const target = event.target;
    this.setState({
      blogText: target.value,
    });
  }

  private handleClosePreview() {
    this.setState({ showModal: false });
  }

  private alertUser(message: string) {
    alert(message);
  }

  private handlePreview(event: React.MouseEvent<HTMLButtonElement>) {
    event.preventDefault();
    const promise = CreatePreview(this.props.username, this.state.title, this.state.blogText);
    promise
      .then((res: any) => {
        const post = res!.body;
        if (post === null) {
          this.alertUser("Could not preview");
          return;
        }
        this.setState({
          post,
          showModal: true,
        });
      })
      .catch((err: any) => {
        let message = err.message;
        if (err.response) {
          message = err.response.text;
        }
        this.alertUser(message);
      });
  }

  private handleSubmitForm(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const promise = CreateArticle(this.props.username, this.state.title, this.state.blogText);
    promise
      .then((res: any) => {
        let message = "Posted article";
        if (res.text) {
          message = res.text;
        }
        this.alertUser(message);
        this.setState({
          blogText: "",
          title: "",
        });
      })
      .catch((err: any) => {
        let message = err.message;
        if (err.response) {
          message = err.response.text;
        }
        this.alertUser(message);
      });
  }
}
