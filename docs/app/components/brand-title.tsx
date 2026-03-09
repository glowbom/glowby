const logoSrc = `${import.meta.env.BASE_URL}glowbom-logo.svg`;

export function BrandTitle() {
  return (
    <span className="inline-flex items-center">
      <img
        alt="Glowbom"
        className="h-[26px] w-auto"
        height={29}
        src={logoSrc}
        width={118}
      />
    </span>
  );
}
