import re

with open('backend/booking-service/main.go', 'r') as f:
    content = f.read()

# Read new createBooking func
with open('patch_create_booking.go', 'r') as f:
    patch = f.read()
    
new_func_match = re.search(r'func \(s \*Server\) createBooking\(w http\.ResponseWriter, r \*http\.Request\) \{.*?\n}\n', patch, re.DOTALL)
new_func = new_func_match.group(0)

# Replace old func
old_content_replaced = re.sub(
    r'func \(s \*Server\) createBooking\(w http\.ResponseWriter, r \*http\.Request\) \{.*?\n}\n\nfunc \(s \*Server\) releaseInventory',
    new_func + '\n\nfunc (s *Server) releaseInventory',
    content,
    flags=re.DOTALL
)

with open('backend/booking-service/main.go', 'w') as f:
    f.write(old_content_replaced)

