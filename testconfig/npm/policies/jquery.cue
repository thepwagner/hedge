// Accept any 3.x release of jquery, published by a trusted set of maintainers

pkg: {
    name: "jquery"
}

version: {
    #stable: !~"(alpha|beta|rc)"
    #version3: =~"^3.*"
    version: #stable & #version3

    #trustedMaintainer: { 
        name: string
        email: "scott.gonzalez@gmail.com" | "dave.methvin@gmail.com" | "4timmywil@gmail.com" | "m.goleb@gmail.com"
        ...
    }
   
    maintainers: [...#trustedMaintainer]
}
