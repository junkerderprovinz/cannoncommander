interface ImportMetaEnv {
  // False in the container build, which leaves out every module only the desktop app loads.
  readonly DESKTOP: boolean;
  // "1" in a preview build, which opens the welcome window on every load.
  readonly VITE_WELCOME_EVERY_LOAD?: string;
}
