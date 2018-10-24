import * as Promise from 'bluebird';
import * as superagent from 'superagent';

export interface IBlogPost {
  global_id: string;
  author: string;
  title: string;
  body: string;
}

const apiURL = "/c2s/feed";

export function GetPublicPosts() {
  return new Promise<IBlogPost[]>((resolve, reject) => {
    superagent
      .get(apiURL)
      .set('Accept', 'application/json')
      .end((error, res) => {
        if (error) {
          reject(error);
          return;
        }
        resolve(res!.body)
      });
  });
}
