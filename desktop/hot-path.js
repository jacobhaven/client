import resolveRoot from './resolve-root'

export default path => {
  return __HOT__ ? `http://localhost:4000/dist/${path}` : resolveRoot('dist', path) // eslint-disable-line no-undef
}
