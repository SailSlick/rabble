import * as React from "react";

import { Redirect } from "react-router-dom";
import * as config from "../../rabble_config.json";
import {
  EditUserPromise, EditUserProfilePicPromise,
  GetUserInfo, IEditUserResult, IUserDetails
} from "../models/edit_user";

interface IAccountEditState {
  bio: string;
  currentPassword: string;
  displayName: string;
  newPassword: string;
  postBodyCss: string;
  postTitleCss: string;
  privateAccount: boolean;
  profilePic: File;
  redirect: boolean;
}

interface IAccountEditProps {
  username: string;
}

export class AccountEdit extends React.Component<IAccountEditProps, IAccountEditState> {
  constructor(props: any) {
    super(props);

    this.state = {
      bio: "",
      currentPassword: "",
      displayName: "",
      newPassword: "",
      postBodyCss: "",
      postTitleCss: "",
      privateAccount: false,
      profilePic: new File([], ""),
      redirect: false,
    };

    this.handlePassword = this.handlePassword.bind(this);
    this.handleNewPassword = this.handleNewPassword.bind(this);
    this.handleProfilePic = this.handleProfilePic.bind(this);
    this.handleBio = this.handleBio.bind(this);
    this.handleDisplayName = this.handleDisplayName.bind(this);
    this.handlePostTitleCss = this.handlePostTitleCss.bind(this);
    this.handlePostBodyCss = this.handlePostBodyCss.bind(this);
    this.handlePrivate = this.handlePrivate.bind(this);
    this.handleUpdate = this.handleUpdate.bind(this);
    this.handleCancel = this.handleCancel.bind(this);
    this.handleUserInfo = this.handleUserInfo.bind(this);
    this.handleGetError = this.handleGetError.bind(this);

    GetUserInfo().then(this.handleUserInfo).catch(this.handleGetError);
  }

  public render() {
    if (this.state.redirect) {
      return (<Redirect to={{ pathname: "/@" + this.props.username  }} />);
    }

    return (
      <div>
        <div className="pure-u-5-24"/>
        <div className="pure-u-14-24">
          <h2>{config.edit_profile_title}</h2>
          {this.form()}
        </div>
      </div>
    );
  }

  public handleUpdate(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    EditUserPromise(
      this.state.bio,
      this.state.displayName,
      this.state.currentPassword,
      this.state.newPassword,
      this.state.privateAccount,
      this.state.postTitleCss,
      this.state.postBodyCss,
    ).then((response: IEditUserResult) => {
      if (!response.success) {
        alert("Error editing: " + response.error);
      } else {
        this.setState({ redirect: true });
      }
    })
    .catch(this.handleUpdateError);
    if (this.state.profilePic.name !== "") {
      // Send a seperate request to handle the profile pic.
      EditUserProfilePicPromise(
        this.state.profilePic
      ).then((response: IEditUserResult) => {
        if (!response.success) {
          alert("Error editing: " + response.error);
        } else {
          this.setState({ redirect: true });
        }
      });
    }
  }

  public handleUserInfo(details: IUserDetails) {
    let isPrivate = false;
    if ("value" in details.private) {
      isPrivate = details.private.value!;
    }

    this.setState({
      bio: details.bio,
      displayName: details.display_name,
      privateAccount: isPrivate,
    });
  }

  private form() {
    return (
      <div>
        <form
          className="pure-form pure-form-aligned"
          onSubmit={this.handleUpdate}
        >
          <fieldset>
            <legend>Blank fields are left unchanged</legend>
            <div className="pure-control-group">
                <label htmlFor="newPassword">{config.new_password}</label>
                <input
                  id="newPassword"
                  type="password"
                  placeholder={config.new_password}
                  className="pure-input-2-3"
                  value={this.state.newPassword}
                  onChange={this.handleNewPassword}
                />
            </div>
            <div className="pure-control-group">
                <label htmlFor="name">Profile Picture</label>
                <input
                  id="profile_pic"
                  accept="image/*"
                  type="file"
                  onChange={this.handleProfilePic}
                />
            </div>
            <div className="pure-control-group">
                <label htmlFor="name">{config.display_name}</label>
                <input
                  id="diplay_name"
                  type="text"
                  placeholder="Display Name"
                  className="pure-input-2-3"
                  value={this.state.displayName}
                  onChange={this.handleDisplayName}
                />
            </div>
            <div className="pure-control-group">
                <label htmlFor="name">{config.bio}</label>
                <textarea
                  id="bio"
                  placeholder="Bio"
                  className="pure-input-2-3 bio-form"
                  value={this.state.bio}
                  onChange={this.handleBio}
                />
            </div>
            <div className="pure-control-group">
                <label htmlFor="name">{"Post Title CSS"}</label>
                <textarea
                  id="post_title_css"
                  placeholder='{"color": "red"}'
                  className="pure-input-2-3 bio-form"
                  value={this.state.postTitleCss}
                  onChange={this.handlePostTitleCss}
                />
            </div>
            <div className="pure-control-group">
                <label htmlFor="name">{"Post Body CSS"}</label>
                <textarea
                  id="post_body_css"
                  placeholder='{"color": "red"}'
                  className="pure-input-2-3 bio-form"
                  value={this.state.postBodyCss}
                  onChange={this.handlePostBodyCss}
                />
            </div>

            <div className="pure-control-group">
              <label htmlFor="private">{config.private_account}</label>
              <input
                id="private"
                type="checkbox"
                checked={this.state.privateAccount}
                onChange={this.handlePrivate}
              />
            </div>

            <legend>Enter your current user account</legend>
            <div className="pure-control-group">
                <label htmlFor="name">{config.password}</label>
                <input
                  id="password"
                  type="password"
                  placeholder="Password"
                  className="pure-input-2-3"
                  required={true}
                  value={this.state.currentPassword}
                  onChange={this.handlePassword}
                />
            </div>

            <div className="pure-control-group">
              <label/>
              <div className="edit-wrapper">
                <button
                  onClick={this.handleCancel}
                  className="pure-button cancel-button edit-button"
                >
                  Cancel
                </button>

                <button
                  type="submit"
                  className="pure-button pure-button-primary primary-button edit-button"
                >
                  {config.update_button}
                </button>
              </div>
            </div>
          </fieldset>
        </form>
      </div>
    );
  }

  private handleCancel(event: React.FormEvent<HTMLButtonElement>) {
    event.preventDefault();
    this.setState({ redirect: true });
  }

  private handleProfilePic(event: React.FormEvent<HTMLInputElement>) {
    const target = event.target as HTMLInputElement;
    const files : FileList | null = target.files;
    if (typeof files != "undefined" && files!.length === 1) {
      const image : File = files![0];
      if (!image.type.startsWith("image/")) {
        return;  // No text files please.
      }
      this.setState({
        profilePic: image,
      });
    }
  }

  private handleNewPassword(event: React.ChangeEvent<HTMLInputElement>) {
    const target = event.target;
    this.setState({
      newPassword: target.value,
    });
  }

  private handlePassword(event: React.ChangeEvent<HTMLInputElement>) {
    const target = event.target;
    this.setState({
      currentPassword: target.value,
    });
  }

  private handleBio(event: React.ChangeEvent<HTMLTextAreaElement>) {
    const target = event.target;
    this.setState({
      bio: target.value,
    });
  }

  private handlePrivate(event: React.ChangeEvent<HTMLInputElement>) {
    const target = event.target;

    this.setState({
      privateAccount: target.checked,
    });
  }

  private handleDisplayName(event: React.ChangeEvent<HTMLInputElement>) {
    const target = event.target;
    this.setState({
      displayName: target.value,
    });
  }

  private handlePostTitleCss(event: React.ChangeEvent<HTMLTextAreaElement>) {
    const target = event.target;
    this.setState({
      postTitleCss: target.value,
    });
  }

  private handlePostBodyCss(event: React.ChangeEvent<HTMLTextAreaElement>) {
    const target = event.target;
    this.setState({
      postBodyCss: target.value,
    });
  }

  private handleUpdateError(e: any) {
    alert("Error attempting to update: " + e.toString());
  }

  private handleGetError(e: any) {
    alert("Error getting user details: " + e.toString());
  }
}
