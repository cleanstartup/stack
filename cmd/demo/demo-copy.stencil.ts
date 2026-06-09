export class DemoCopy {
  static headline() {
    return "hello from stack";
  }

  static intro() {
    return "This page loads global styles and Stencil components.";
  }

  static hint(clicked: boolean) {
    return clicked ? "clicked!" : "Open the console and click the button.";
  }
}
