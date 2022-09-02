# hedge

Proxying package registry and walled garden perimeter.

- should the server be live, or rendering to static hosting?
- live initially, with the ability to render "planned"
- todo: layers of caching
    - raw results from remotes
    - filtered results

- uh-oh, this is still pretty slow
- let's move to static rendering!
- have a CLI push pre-rendered files into redis
- have the "server" dumbly serve files from redis
- eventually, push changes to servers for performance

- I CAN FIX IT!
    - Faster encoding of large cache payloads (ProtoBuf)
    - Generic cache interface, for clarity/productivity
    - Handroll the mapstructure bits
    - This is a lot of pretty boring code
    