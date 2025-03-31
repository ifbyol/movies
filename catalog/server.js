import got from "got";
import hpropagate from "hpropagate";
import express from "express"
import mongodb from "mongodb";

hpropagate({
  setAndPropagateCorrelationId: true,
  headersToPropagate: ["baggage.okteto-divert"],
});

// const express = require("express");
const mongo = mongodb.MongoClient;

const app = express();

const oktetoDivertHeader = "baggage.okteto-divert";

const url = `mongodb://${process.env.MONGODB_USERNAME}:${encodeURIComponent(process.env.MONGODB_PASSWORD)}@${process.env.MONGODB_HOST}:27017/${process.env.MONGODB_DATABASE}`;

async function callAPI() {
  const url = `http://api:8080/users`;
  console.log(`calling api service`);

  return await got(url).text();
}

function startWithRetry() {
  mongo.connect(url, { 
    useUnifiedTopology: true,
    useNewUrlParser: true,
    connectTimeoutMS: 1000,
    socketTimeoutMS: 1000,
  }, (err, client) => {
    if (err) {
      console.error(`Error connecting, retrying in 1 sec: ${err}`);
      setTimeout(startWithRetry, 1000);
      return;
    }

    const db = client.db(process.env.MONGODB_DATABASE);

    app.listen(8080, () => {
      app.get("/catalog/healthz", (req, res, next) => {
        res.sendStatus(200)
        return;
      });

      app.get("/catalog", async (req, res, next) => {
        console.log('Request headers:', req.headers);
        console.log(`GET /catalog`)

        const response = await callAPI();
        console.log(`response from api service: ${response}`);

        db.collection('catalog').find().toArray( (err, results) =>{
          if (err){
            console.log(`failed to query movies: ${err}`)
            res.json([]);
            return;
          }
          res.json(results);
        });
      });

      console.log("Server running on port 8080.");
    });
  });
};

startWithRetry();