import yaml
from config_defaults import default_config_dict as config_dict

mydict = {
    'testing_append': {
        'test_id' : '2'
    }
}

with open('/home/tinker/code/lyvecloudfuse/config.yaml', 'r') as file:
    test_config = yaml.safe_load(file)

print(test_config)

# del test_config['file_cache']

# if 'file_cache' in test_config:
#     test_config['file_cache'].update(mydict)
# else:
#     test_config['filecache'] = mydict

#print(test_config['file_cache'])

with open('/home/tinker/code/lyvecloudfuse/testWrite_config.yaml', 'w') as file:
    yaml.safe_dump(config_dict, file, sort_keys=False)