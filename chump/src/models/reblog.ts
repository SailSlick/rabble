import * as request from "superagent";

interface IReblogPostBody {
  article_id: number;
}

export function SendReblog(articleId: number) {
  const endpoint: string = "/c2s/announce";
  const postBody: IReblogPostBody = {
    article_id: articleId,
  };
  return new Promise((resolve, reject) => {
    request
      .post(endpoint)
      .set("Content-Type", "application/json")
      .set("Accept", "application/json")
      .send(postBody)
      .retry(2)
      .end((error, res) => {
        if (error) {
          reject(error);
          return;
        }

        const resp = res!.body;
        if (res.status !== 200) {
          reject(new Error("status = " + res.status.toString()));
        }

        resolve(res);
      });
  });
}
