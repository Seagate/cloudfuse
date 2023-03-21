default_config_dict = {
    'foreground': False, 
    'allow-other': True, 
    'logging': {
        'level': 'log_debug', 
        'file-path': 'lyvecloudfuse-logs.txt', 
        'type': 'base'},
    'components':[
        'libfuse', 
        'file_cache', 
        'attr_cache', 
        'azstorage'], 
    'libfuse': {
        'default-permission': 511, 
        'attribute-expiration-sec': 120, 
        'entry-expiration-sec': 120, 
        'negative-entry-expiration-sec': 240, 
        'ignore-open-flags': True},
    'file_cache': {
        'path': '',
        'timeout-sec': 240,
        'max-size-mb': 4096,
        'create-empty-file': True},
    'attr_cache': {
        'timeout-sec': 7200},
    'azstorage': {
        'type': 'block',
        'account-name':'',
        'account-key': '',
        'endpoint': '',
        'mode': 'key',
        'container': ''}
}
