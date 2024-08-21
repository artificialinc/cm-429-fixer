# cert-manager 429 fixer

Simple operator that watchers cert-manager orders and challenges and resets them to pending if they have errored with a 429 status code.

Prototyping a possible fix for [this issue](https://github.com/cert-manager/cert-manager/issues/5867)
