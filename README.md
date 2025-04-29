# Golang WebSocket Implementation for learning and future use. 

## Why not use Gorrila?
The idea for this implementation is to be usable in my other future projects and to be customisable by me. Ill also learn some networking/ TCP/IP and specifically RFC 6455. I will *hopefully* have a better time implementing any RFCs in the future

## How to use?
Currently this works by implementing a http webserver with an endpoint that is used to upgrade the http connections to WebSocket connections.
Once the websocket connection is formed the connection can be maintained until either side decides to terminate the connections. 

Will expose APIs for server and client use as the testing is completed. 

## Current State
The implementation is in a state where it should be usable, but has not been tested, or stress tested. 
Currently testing the websocket implementation against the autobahn websocket tester to figure out any issues. 
Binary and Text frames seems to be working but not in large packet sizes, currently re-working and implementing. 

## Use Case
This WS implementation will be used in another project a local video stream from an embedded device to a PC. It can be extended further as more is implemented

## Future Work
- Add Handling of Ping/Pong Opcodes
- Allow fragmentation of packets.
- Look into potential extensibility negotiations
- Look into security considerations and implement them.
