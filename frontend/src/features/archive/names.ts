// The name of an archive without its ending, which is also the folder it unpacks into.
export const stripArchive = (name: string) => name.replace(/\.(tar\.(zst|gz|xz|bz2)|zip|7z|rar|tgz|tar|zst|gz)$/i, '');
