interface ImportMetaEnv {
  // False in the container build, which leaves out every module only the desktop app loads.
  readonly DESKTOP: boolean;
}
