import yaml

with open('/home/tinker/code/lyvecloudfuse/config.yaml', 'r') as file:
    test_config = yaml.safe_load(file)

test_config['file_cache']['timeout-sec'] = 240

print(test_config['file_cache']['timeout-sec'])

with open('/home/tinker/code/lyvecloudfuse/testWrite_config.yaml', 'w') as file:
    yaml.safe_dump(test_config, file, sort_keys=False)