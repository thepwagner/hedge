# hedge
Proxying package registry and walled garden perimeter.

What are we doing here?

- define a gitops layout for a proxying package registry
    - assume tools+ci operate on that gitops repo for dependency ingress
- use debian as the first type of package hosted

should the server be live, or rendering to static hosting?
live initially, with the ability to render "planned"

TODO:
- PoC package metadata
- tools to build the gitops layout from an upstream source
