import * as React from "react";
import { render } from "react-dom";
import { HashRouter, Route, Switch } from "react-router-dom";

import {About} from "./components/about";
import {Header} from "./components/header";
import {Home} from "./components/home";
import {Write} from "./components/write";

require("./styles/site.css"); // tslint:disable-line

const App = () => (
  <HashRouter>
    <div>
      <Header/>
      <Switch>
        <Route exact={true} path="/" component={Home}/>
        <Route path="/about" component={About}/>
        <Route path="/write" component={Write}/>
      </Switch>
    </div>
  </HashRouter>
);

render(<App />, document.getElementById("root"));
