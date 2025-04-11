const express = require("express");
const mongo = require("mongodb").MongoClient;

const app = express();

const url = `mongodb://${process.env.MONGODB_USERNAME}:${encodeURIComponent(process.env.MONGODB_PASSWORD)}@${process.env.MONGODB_HOST}:27017/${process.env.MONGODB_DATABASE}`;

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

      app.get("/catalog", (req, res, next) => {
        console.log(`GET /catalog`)
        
        // Log all headers to verify baggage header propagation
        console.log("Request headers:");
        console.log(JSON.stringify(req.headers, null, 2));
        
        // Specifically log the baggage header if it exists
        if (req.headers.baggage) {
          console.log(`Received baggage header: ${req.headers.baggage}`);
        } else {
          console.log("No baggage header received");
        }
        
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