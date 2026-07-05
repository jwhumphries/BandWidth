// Footer is a small, unobtrusive copyright link shown on every page,
// authenticated or not, since it sits in App.tsx above the route tree.
export default function Footer() {
  return (
    <a
      href="https://github.com/jwhumphries"
      target="_blank"
      rel="noopener noreferrer"
      className="text-base-content/30 hover:text-base-content/60 fixed right-3 bottom-2 z-10 text-xs transition-colors"
    >
      © {new Date().getFullYear()} John Humphries
    </a>
  );
}
