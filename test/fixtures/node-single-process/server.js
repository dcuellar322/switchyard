const http = require("node:http");

const port = Number(process.env.PORT);
const server = http.createServer((_request, response) => {
  response.writeHead(200, { "content-type": "text/plain" });
  response.end("switchyard npm fixture\n");
});

server.listen(port, "127.0.0.1", () => {
  console.log(`info npm fixture listening on ${port}`);
  console.error("warning npm fixture stderr ready");
});
