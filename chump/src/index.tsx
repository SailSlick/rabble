import * as React from "react";
import {render} from "react-dom";
import {Link, Route, HashRouter, Switch} from "react-router-dom";
import { toast } from 'react-toastify';

import * as config from "../rabble_config.json";

import {PrivateRoute} from "./proute";
import {About} from "./components/about";
import {AccountEdit} from "./components/account/account_edit";
import {Edit} from "./components/article/edit";
import {HeaderWithRouter} from "./components/header";
import {Pending} from "./components/follow/pending";
import {Feed} from "./components/feed";
import {Register} from "./components/account/register";
import {Write} from "./components/article/write";
import {Login} from "./components/account/login";
import {Logout} from "./components/account/logout";
import {UserFeed} from "./components/user_feed";
import {Follow} from "./components/follow/follow";
import {SinglePost} from "./components/article/single_post";
import {SearchResults} from "./components/search_results";
import {UserProfile} from "./components/account/user_profile";
import {RecommendedPosts} from "./components/recommended_posts";

import { SendView } from "./models/view";

require("./styles/site.css"); // tslint:disable-line
require("react-toastify/dist/ReactToastify.css"); // tslint:disable-line

// IAppState is top level state.
// Don't put state that might change often here.
interface IAppState {
  username: string;
  userId: number;
}

const LOCAL_STORAGE_USERNAME : string = "username";
const LOCAL_STORAGE_USERID : string = "userid";

toast.configure({
  autoClose: 4000,
  draggable: false,
  position: toast.POSITION.BOTTOM_RIGHT,
});

export class App extends React.Component<{}, IAppState> {
  constructor(props: {}) {
    super(props);

    this.state = {
      username: this.getUsername(),
      userId: this.getUserId(),
    }

    this.getUsername = this.getUsername.bind(this);
    this.login = this.login.bind(this);
    this.logout = this.logout.bind(this);
    this.trackView = this.trackView.bind(this);
  }

  getUsername() : string {
    if (!localStorage.hasOwnProperty(LOCAL_STORAGE_USERNAME)) {
      return "";
    }
    return localStorage.getItem(LOCAL_STORAGE_USERNAME)!;
  }

  getUserId() : number {
    if (!localStorage.hasOwnProperty(LOCAL_STORAGE_USERID)) {
      return 0;
    }
    return parseInt(localStorage.getItem(LOCAL_STORAGE_USERID)!);
  }

  login(username: string, userId: number) {
    this.setState({
      username,
      userId
    });
    localStorage.setItem(LOCAL_STORAGE_USERNAME, username);
    localStorage.setItem(LOCAL_STORAGE_USERID, userId.toString());
  }

  logout() {
    this.setState({
      username: "",
      userId: 0,
    });
    localStorage.removeItem(LOCAL_STORAGE_USERNAME);
    localStorage.removeItem(LOCAL_STORAGE_USERID);
  }

  trackView() {
    const path = window.location.hash;
    if (path === "") {
        // Do not log the empty path shown on first load, log instead the
        // hash path that it is immediately redirected to.
        return;
    }
    SendView(path);
  }

  componentDidMount() {
    if (config.track_views) {
      window.addEventListener("hashchange", this.trackView);
    }
  }

  render() {
    if (config.track_views) {
        // Must manually log the view the first time,
        // as only hash *changes* trigger a log.
        this.trackView();
    }
    return (
      <HashRouter>
        <div>
          <HeaderWithRouter
            username={this.state.username}
            userId={this.state.userId}
          />
          <Switch>
            <Route
              exact={true}
              path="/"
              render={(props) => <Feed {...props} queryUserId={0} username={this.state.username} />}
            />
            <Route path="/about" component={About}/>
            <Route
              path="/@:user/:article_id"
              render={(props) => <SinglePost {...props} username={this.state.username} />}
            />
            <Route
              path="/@:user"
              render={(props) =>
                <UserProfile {...props}
                  username={this.state.username}
                  userId={this.state.userId}
               />}
            />
            <Route
              path="/login"
              render={(props) => <Login {...props} loginCallback={this.login} />}
            />
            <Route
              path="/logout"
              render={(props) => <Logout {...props} logoutCallback={this.logout} />}
            />
            <Route
              path="/register"
              render={(props) => <Register {...props} loginCallback={this.login} />}
            />
            <Route
              path="/search/:query"
              render={(props) => <SearchResults {...props} username={this.state.username} />}
            />
            <PrivateRoute
              path="/feed"
              queryUserId={this.state.userId}
              username={this.state.username}
              component={Feed}
            />
            <PrivateRoute
              path="/recommended_posts"
              queryUserId={this.state.userId}
              username={this.state.username}
              component={RecommendedPosts}
            />
            <PrivateRoute
              path="/follow"
              username={this.state.username}
              userId={this.state.userId}
              component={Follow}
            />
            <PrivateRoute
              path="/@/pending"
              username={this.state.username}
              component={Pending}
            />
            <PrivateRoute
              path="/@/edit"
              username={this.state.username}
              component={AccountEdit}
            />
            <PrivateRoute
              path="/write/"
              username={this.state.username}
              component={Write}
            />
            <PrivateRoute
              path="/edit/:article_id"
              username={this.state.username}
              component={Edit}
            />
          </Switch>
        </div>
      </HashRouter>
    );
  }
};

render(<App />, document.getElementById("root"));
