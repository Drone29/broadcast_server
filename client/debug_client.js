// Execute in browser console

const ws = new WebSocket("ws://localhost:1234/ws");

ws.onopen = () => {
    console.log("Connected to WebSocket server");
    ws.send("Hello from client!");
};

ws.onmessage = (event) => {
    console.log("Received:", event.data);
};

// Send a message every 5 seconds
setInterval(() => {
    ws.send("Ping from client!");
}, 500);

ws.onclose = () => {
    console.log("Disconnected from server");
};