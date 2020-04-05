import * as React from "react";
import { Link } from "react-router-dom";
import * as request from "superagent";

import * as config from "../../../rabble_config.json";
import { CreateFollow, CreateRssFollow } from "../../models/follow";
import { RootComponent } from "../root_component";

interface IFormState {
  placeholder: string;
  toFollow: string;
  type: string;
}

export interface IFormProps {
  username: string;
  userId: number;
}

const username_placeholder = "user[@instance.com]";
const feed_placeholder = "https://examplesite.com/feed";
const username_type = "username";
const feed_type = "feed";

export class FollowForm extends RootComponent<IFormProps, IFormState> {
  constructor(props: IFormProps) {
    super(props);

    this.state = {
      placeholder: username_placeholder,
      toFollow: "",
      type: username_type,
    };

    this.handleInputChange = this.handleInputChange.bind(this);
    this.handleDropdownChange = this.handleDropdownChange.bind(this);
    this.handleSubmitForm = this.handleSubmitForm.bind(this);
  }

  public render() {
    return (
      <form className="pure-form pure-form-aligned" onSubmit={this.handleSubmitForm}>
        <div className="pure-control-group">
          <input
            type="text"
            name="toFollow"
            value={this.state.toFollow}
            onChange={this.handleInputChange}
            className="pure-input-1-2 blog-input"
            placeholder={this.state.placeholder}
          />
          <label>
            <select
              id="type"
              className="pure-input-2-5"
              onChange={this.handleDropdownChange}
              value={this.state.type}
            >
                <option value={username_type}>{config.username}</option>
                <option value={feed_type}>Rss/Atom</option>
            </select>
          </label>
        </div>
        <button
          type="submit"
          className="pure-button pure-input-1-3 pure-button-primary primary-button"
        >
          {config.follow_text}
        </button>
      </form>
    );
  }

  private handleInputChange(event: React.ChangeEvent<HTMLInputElement>) {
    const target = event.target;
    this.setState({
      toFollow: target.value,
    });
  }

  private handleDropdownChange(event: React.ChangeEvent<HTMLSelectElement>) {
    let placeholder = username_placeholder;
    if (event.target.value == feed_type) {
      placeholder = feed_placeholder;
    }

    this.setState({
      placeholder,
      type: event.target.value,
    });
  }

  private handleSubmitForm(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const promise = (this.state.type === feed_type)
      ? CreateRssFollow(this.props.username, this.state.toFollow)
      : CreateFollow(this.props.username, this.state.toFollow, "");

    promise.then((res: request.Response) => {
      if (res.status !== 200) {
        this.errorToast({ statusCode: res.status });
      } else {
        this.successToast(config.success_follow_form);
      }

      this.setState({
        toFollow: "",
        type: username_type,
      });
    })
    .catch((err: Error) => {
      this.errorToast({ debug: err });
    });
  }
}
