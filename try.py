import requests

# Test for .msg file
url = "http://localhost:80/msg-to-txt"
files = {'file': open('/Users/konsultera/Documents/projects/pdf-converter/LamdaTest_UnitedLayer_nbarat_native_3_ekmt001_00014409.msg', 'rb')}
headers = {"X-API-Key": "0f4b7a98e3c14f66b7d3a9c1e8f2bde1c86c59d8a73245fc9edb1a7df9246c5a"}

response = requests.post(url, files=files, headers=headers)
print(response.content)
with open('test.txt', 'wb') as f:
    f.write(response.content)

# Test for .doc file
url_doc = "http://localhost:80/doc-to-txt"
files_doc = {'file': open('/Users/konsultera/Documents/projects/pdf-converter/MystiqueAI_Partner_GTM_Framework_India.doc', 'rb')}

response_doc = requests.post(url_doc, files=files_doc, headers=headers)
print(response_doc.content)
with open('test_doc.txt', 'wb') as f_doc:
    f_doc.write(response_doc.content)

# Test for .pptx file
url_pptx = "http://localhost:80/convert"  # Assuming the endpoint exists
files_pptx = {'file': open('/Users/konsultera/Documents/projects/pdf-converter/Integreon-DataLake-Approach-Template.pptx', 'rb')}

response_pptx = requests.post(url_pptx, files=files_pptx, headers=headers)
print(response_pptx.content)
with open('test_pptx.pdf', 'wb') as f_pptx:
    f_pptx.write(response_pptx.content)