const http = require("http");

const PORT = 3000;

console.log("==================================================");
console.log("🤖 AI Agent Webhook Listener (Sleeping on Port " + PORT + ")");
console.log("==================================================");

const server = http.createServer((req, res) => {
  if (req.method === "POST" && req.url === "/trade") {
    let body = "";

    req.on("data", (chunk) => {
      body += chunk.toString();
    });

    req.on("end", () => {
      const payload = JSON.parse(body);
      console.log("\n⚡ WAKE UP CALL RECEIVED!");
      console.log("----------------------------------------");
      console.log("Time: " + new Date(payload.timestamp * 1000).toISOString());
      console.log("Node: " + payload.node_id);
      console.log("Executing Trade:");
      console.log("   Pair: " + payload.signal.payload.token_pair);
      console.log("   Action: " + payload.signal.payload.direction);
      console.log("   Entry: " + payload.signal.payload.entry_price);
      console.log("----------------------------------------\n");

      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ status: "success", message: "Trade Executed" }));
    });
  } else {
    res.writeHead(404);
    res.end();
  }
});

server.listen(PORT);
