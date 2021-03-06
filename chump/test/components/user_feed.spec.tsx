import * as Promise from "bluebird";
import { expect } from "chai";
import * as React from "react";
import * as ReactDOM from "react-dom";
import { MemoryRouter } from "react-router";
import * as sinon from "sinon";

import { UserFeed } from "../../src/components/user_feed";
import { IParsedPost } from "../../src/models/posts";
import { mount, shallow } from "./enzyme";

describe("User", () => {
  it("should call post collecting methods", () => {
    const getPosts = sinon.spy(UserFeed.prototype, "getPosts");
    const render = sinon.spy(UserFeed.prototype, "renderPosts");

    const userProps = {
      userId: 0,
      username: "",
      viewing: "cian",
    };
    const wrapper = mount(
      <MemoryRouter>
        <UserFeed {...userProps} />
      </MemoryRouter>,
    );

    expect(getPosts).to.have.property("callCount", 1);
    expect(render).to.have.property("callCount", 1);

    // Cleanup spies
    getPosts.restore();
    render.restore();
  });

  it("should properly render posts", () => {
    const userProps = {
      userId: 0,
      username: "",
      viewing: "sips",
    };
    const wrapper = shallow(<UserFeed {...userProps} />);
    expect(wrapper.find("div")).to.have.lengthOf(4);
    expect(wrapper.find("Card")).to.have.lengthOf(0);

    wrapper.setState({
      publicBlog: [{
        author: "sips",
        body: "id be in so much trouble<br>i'd never live it down<br>lol",
        title: "the man, the myth, the legend",
      }],
      ready: true,
    });

    expect(wrapper.find("div")).to.have.lengthOf(2);
    expect(wrapper.find("Card")).to.have.lengthOf(1);
  });
});
