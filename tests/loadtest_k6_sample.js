// Copyright 2025 momentics@gmail.com
// Apache License 2.0
//
// Example k6 script for WebSocket performance/load testing of hioload-ws server.
// Stages: ramp up 1000 VUs, sustain, ramp down.

import ws from 'k6/ws';
import { check } from 'k6';

export let options = {
    stages: [
        { duration: '10s', target: 1000 },
        { duration: '1m', target: 1000 },
        { duration: '10s', target: 0 },
    ],
};

export default function () {
    var url = 'ws://localhost:8080/ws';
    var res = ws.connect(url, function (socket) {
        socket.on('open', function () {
            // Send 10 messages per connection
            for (let i = 0; i < 10; i++) {
                socket.send('loadtest message ' + i);
            }
        });
        socket.on('message', function (data) {
            check(data, {
                'server echoed message': (msg) => msg.indexOf('loadtest') >= 0,
            });
        });
        socket.on('close', function () {});
        socket.setTimeout(function () {
            socket.close();
        }, 5000);
    });
    check(res, { 'Connected successfully': (r) => r && r.status === 101 });
};
