with import <nixpkgs>{};
stdenv.mkDerivation rec {
    name = "LED";
    buildInputs =  [ autoreconfHook pkgconfig cmake ncurses go libtsm ];
    shellHook = ''
        GODEBUG=cgocheck=0 go run *.go -cpuprofile mux.prof
        exit
    '';

    GOPATH="/home/ajanse/.go";
}
