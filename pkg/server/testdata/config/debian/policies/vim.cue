// Allow basic vim and dependencies

#vimPackage: =~"^vim(|-common|-runtime)$"
#vimDependency: "xxd" | "libgpm2"

name: #vimPackage | #vimDependency
