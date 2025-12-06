# JSON and String Methods Example

This example demonstrates the new JSON and string methods in the hioload-ws high-level API.

## Available Routes

- `GET /echo` - Echo WebSocket endpoint using `ReadString` and `WriteString`
- `GET /json` - JSON WebSocket endpoint using `ReadJSON` and `WriteJSON`
- `GET /mixed` - Mixed endpoint using both string and JSON methods

## Usage

```bash
go run examples/highlevel/json_string_methods/main.go
```

The server will listen on `:8080` and handle connections with the new methods.

## Key Features Demonstrated

1. **ReadString/WriteString** - Reading and writing UTF-8 string messages
2. **ReadJSON/WriteJSON** - Marshaling/unmarshaling JSON data directly
3. **Mixed Usage** - Combining string and JSON methods in the same connection
4. **Error Handling** - Proper error handling for all new methods
5. **Integration** - Works seamlessly with existing features (routes, params, etc.)