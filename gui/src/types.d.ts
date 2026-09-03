declare module "world-map-country-shapes" {
  export interface CountryShape {
    id: string;
    shape: string;
  }
  const countryShapes: CountryShape[];
  export default countryShapes;
}
