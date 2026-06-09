export class DemoCopy {
  static headline() {
    return "hello from site";
  }

  static intro() {
    return "This static site uses Hugo for rendering and the same Tailwind and Stencil pipeline as the app target.";
  }

  static hint(clicked: boolean) {
    return clicked
      ? "clicked!"
      : "The component stays interactive even on the static site.";
  }
}
