import os
import random
import string
import time
import math
from locust import HttpUser, task, constant_pacing, events
from locust.shape import LoadTestShape

# --- Global settings for custom arguments ---
# This will be populated by the init_parser event listener
settings = {
    "num_users": 1000,  # Default, should match the initialized graph
    "total_duration": 300, # Default: 10 minutes
    "cycle_duration": 300, # Default: 5 minute cycle
    "min_users": 10,
    "max_users": 500,
}

@events.init_command_line_parser.add_listener
def on_init_parser(parser):
    """
    Add custom arguments to the Locust command line.
    These are stored in the global `settings` dict.
    """
    parser.add_argument("--num-users", type=int, env_var="LOCUST_NUM_USERS", default=settings["num_users"], help="Total number of users in the social graph (for mentions)")
    parser.add_argument("--total-duration", type=int, env_var="LOCUST_TOTAL_DURATION", default=settings["total_duration"], help="Total experiment duration in seconds")
    parser.add_argument("--cycle-duration", type=int, env_var="LOCUST_CYCLE_DURATION", default=settings["cycle_duration"], help="Duration of one diurnal cycle in seconds")
    parser.add_argument("--min-users", type=int, env_var="LOCUST_MIN_USERS", default=settings["min_users"], help="Minimum number of concurrent users in a cycle")
    parser.add_argument("--max-users", type=int, env_var="LOCUST_MAX_USERS", default=settings["max_users"], help="Maximum number of concurrent users in a cycle")

@events.init.add_listener
def on_init(environment, **kwargs):
    """
    Read custom arguments and store them in the global settings dict.
    This runs once when the test is initialized.
    """
    global settings
    settings["num_users"] = environment.parsed_options.num_users
    settings["total_duration"] = environment.parsed_options.total_duration
    settings["cycle_duration"] = environment.parsed_options.cycle_duration
    settings["min_users"] = environment.parsed_options.min_users
    settings["max_users"] = environment.parsed_options.max_users
    print("--- Locust settings loaded ---")
    print(f"Total users in graph: {settings['num_users']}")
    print(f"Experiment duration: {settings['total_duration']}s")
    print(f"Diurnal cycle duration: {settings['cycle_duration']}s")
    print(f"User range: {settings['min_users']} -> {settings['max_users']}")
    print("----------------------------")


class DiurnalShape(LoadTestShape):
    """
    A load shape that simulates a diurnal (day/night) pattern.
    The user count follows a sine wave between min_users and max_users.
    The test stops after total_duration seconds.
    """

    def tick(self):
        run_time = self.get_run_time()

        if run_time > settings["total_duration"]:
            return None # Stop the test

        # A cosine wave is used to start at the minimum number of users,
        # peak at the middle of the cycle, and return to the minimum.
        amplitude = (settings["max_users"] - settings["min_users"]) / 2
        midpoint = settings["min_users"] + amplitude

        rads = (2 * math.pi * run_time) / settings["cycle_duration"]
        user_count = midpoint - amplitude * math.cos(rads)

        # To make the actual user count track the target shape as closely as possible,
        # we calculate a dynamic spawn rate. It's the absolute difference between
        # the target user count and the current user count. This tells Locust to
        # try to close the gap in a single step.
        spawn_rate = max(abs(round(user_count) - self.get_current_user_count()), 1)

        return (round(user_count), spawn_rate)


class SocialUser(HttpUser):
    """
    User that composes posts on the social network.
    """
    # To achieve a target of ~1000 req/s with the default 100 max_users,
    # each user must generate 10 requests per second. This means each
    # task execution cycle (task + wait) should take 0.1 seconds.
    # The `constant_pacing` wait time is perfect for this, as it ensures
    # a task runs every N seconds, automatically subtracting the task's execution time.
    wait_time = constant_pacing(0.5)

    @task
    def compose_post(self):
        """
        Simulates a user composing and saving a new post with text and optional images.
        This is based on the `upload_save` function in `init_social_graph.py`.
        """
        # 1. Pick a random user ID from the total number of registered users
        user_id = str(random.randint(0, settings["num_users"] - 1))

        # 2. Generate text content
        text = ''.join(random.choices(string.ascii_letters + string.digits, k=256))
        # Add random user mentions
        for _ in range(random.randint(0, 5)):
            text += ' @username_' + str(random.randint(0, settings["num_users"] - 1))
        # Add random urls
        for _ in range(random.randint(0, 5)):
            text += ' http://' + ''.join(random.choices(string.ascii_lowercase + string.digits, k=64))

        # 3. Prepare form data and files
        form_data = {
            'user_id': user_id,
            'text': text
        }

        files_to_upload = []
        num_images = random.randint(1, 4)
        for i in range(num_images):
            dummy_image_data = os.urandom(1024)  # 1KB of random data
            files_to_upload.append(
                ('images', (f'image_{i}.jpg', dummy_image_data, 'image/jpeg'))
            )

        # 4. Send the POST request to the /save endpoint
        with self.client.post(
            "/save",
            data=form_data,
            files=files_to_upload,
            name="/save [compose_post]", # Group stats under a more descriptive name
            catch_response=True # Important: allows us to check the response
        ) as response:
            if not response.ok:
                print(f"Request to /save failed with status {response.status_code}: {response.text}")
                response.failure(f"Status code {response.status_code}")
            else:
                response.success()