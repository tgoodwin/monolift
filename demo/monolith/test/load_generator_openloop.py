import asyncio
import aiohttp
import time
import math
import random
import string
import os
import argparse


class DiurnalShape:
    """
    A load shape that simulates a diurnal (day/night) pattern for RPS.
    The request rate follows a sine wave between min_rps and max_rps, with
    stable low-load periods at the beginning and end of the test.
    """

    def __init__(self, min_rps, max_rps, cycle_duration, total_duration, warmup_pct=0.30):
        self.min_rps = min_rps
        self.max_rps = max_rps
        self.cycle_duration = cycle_duration
        self.total_duration = total_duration
        self.start_time = time.time()

        # Define the test phases based on a percentage of the total duration.
        self.warmup_end_time = warmup_pct * self.total_duration
        self.ramp_end_time = (1 - warmup_pct) * self.total_duration

    def get_run_time(self):
        return time.time() - self.start_time

    def tick(self):
        """
        Calculates the target RPS for the current time, with stable low-load
        periods at the start and end of the total duration.
        """
        run_time = self.get_run_time()
        if run_time > self.total_duration:
            return None  # Stop the test


        # Phase 1: Warm-up period (stable low load)
        if run_time < self.warmup_end_time:
            return self.min_rps

        # Phase 3: Cool-down period (stable low load)
        if run_time > self.ramp_end_time:
            return self.min_rps

        # Phase 2: Ramp-up and ramp-down period (the sine wave)
        # The wave is compressed into the middle of the test duration.
        ramp_duration = self.ramp_end_time - self.warmup_end_time
        time_into_ramp = run_time - self.warmup_end_time

        # A cosine wave starts at the minimum, peaks in the middle, and returns to the minimum.
        amplitude = (self.max_rps - self.min_rps) / 2
        midpoint = self.min_rps + amplitude

        # The full 2*pi cycle of the wave completes over the ramp_duration
        # to ensure a smooth transition back to min_rps for the cool-down phase.
        rads = (2 * math.pi * time_into_ramp) / ramp_duration
        rps = midpoint - amplitude * math.cos(rads)
        return round(rps)


def generate_compose_post_data(num_total_users):
    """
    Creates an aiohttp.FormData object for a compose post request.
    This logic is adapted from init_social_graph.py and locustfile.py.
    """
    data = aiohttp.FormData()

    # 1. Pick a random user ID
    user_id = str(random.randint(0, num_total_users - 1))

    # 2. Generate text content
    text = ''.join(random.choices(string.ascii_letters + string.digits, k=256))
    # Add random user mentions
    for _ in range(random.randint(0, 5)):
        text += ' @username_' + str(random.randint(0, num_total_users - 1))
    # Add random urls
    for _ in range(random.randint(0, 5)):
        text += ' http://' + ''.join(random.choices(string.ascii_lowercase + string.digits, k=64))

    data.add_field('user_id', user_id)
    data.add_field('text', text)

    # 3. Generate and add dummy image files
    num_images = random.randint(1, 4)
    for i in range(num_images):
        dummy_image_data = os.urandom(1024)  # 1KB of random data
        data.add_field(
            'images',
            dummy_image_data,
            filename=f'image_{i}.jpg',
            content_type='image/jpeg'
        )
    return data


async def send_request(session, url, num_total_users, results_queue):
    """Sends a single request and puts its status code in the results queue."""
    try:
        data = generate_compose_post_data(num_total_users)
        # The `timeout` here prevents a single slow request from holding a connection indefinitely.
        # The generator will continue firing new requests even if old ones are slow.
        async with session.post(url, data=data, timeout=aiohttp.ClientTimeout(total=30)) as response:
            # We don't need the response body, just the status.
            # This is a "fire-and-forget" action from the perspective of the main loop.
            await results_queue.put(response.status)
    except Exception as e:
        # Put the exception in the queue to be counted as a failure.
        await results_queue.put(e)


async def process_results(results_queue, total_duration, status_tracker):
    """
    Runs in the background, consuming from the queue and printing stats.
    """
    start_time = time.time()
    last_print_time = start_time

    # Cumulative stats for the whole run
    total_success_count = 0
    total_failure_count = 0
    status_counts = {}

    # Per-interval stats for periodic reporting
    interval_req_count = 0

    print("Starting results processor...")

    while time.time() - start_time < total_duration + 5:  # Run for 5 extra secs to catch trailing results
        try:
            # Wait for a result, but with a timeout so we can print stats periodically.
            result = await asyncio.wait_for(results_queue.get(), timeout=1.0)
            interval_req_count += 1
            if isinstance(result, int):
                if 200 <= result < 300:
                    total_success_count += 1
                else:
                    total_failure_count += 1
                status_counts[result] = status_counts.get(result, 0) + 1
            else:  # It's an exception
                total_failure_count += 1
                exc_name = type(result).__name__
                status_counts[exc_name] = status_counts.get(exc_name, 0) + 1

        except asyncio.TimeoutError:
            # This is expected when the queue is empty.
            if time.time() - start_time > total_duration + 2:
                break  # Stop if the main loop is done and the queue has been empty for a bit.

        # Print stats every 5 seconds
        current_time = time.time()
        interval_duration = current_time - last_print_time
        if interval_duration >= 5.0:
            run_time = int(current_time - start_time)

            # Calculate RPS for the last interval, not the cumulative average
            interval_rps = interval_req_count / interval_duration if interval_duration > 0 else 0

            print(
                f"Time: {run_time:3d}s | "
                f"Target RPS: {status_tracker['target_rps']:4d} | "
                f"Interval RPS: {interval_rps:7.1f} | "
                f"Total Success: {total_success_count:6d} | "
                f"Total Fail: {total_failure_count:5d}"
            )
            # Reset for the next interval
            last_print_time = current_time
            interval_req_count = 0

    print("\n--- Final Results ---")
    print(f"Total requests: {total_success_count + total_failure_count}")
    print(f"  - Successful: {total_success_count}")
    print(f"  - Failed:     {total_failure_count}")
    if total_failure_count > 0:
        print("Failure details (by status code or exception):")
        for status, count in sorted(status_counts.items(), key=lambda item: str(item[0])):
            if not (isinstance(status, int) and 200 <= status < 300):
                print(f"  - {status}: {count} times")


async def main(args):
    """The main function to orchestrate the load generation."""
    shape = DiurnalShape(args.min_rps, args.max_rps, args.cycle_duration, args.total_duration)
    results_queue = asyncio.Queue()
    status_tracker = {'target_rps': 0}
    url = f"http://{args.ip}:{args.port}/save"

    print("--- True Open-Loop Load Generator ---")
    print(f"Target URL: {url}")
    print(f"Total duration: {args.total_duration}s, Cycle duration: {args.cycle_duration}s")
    print(f"RPS Range: {args.min_rps} -> {args.max_rps}")
    print("---------------------------------------")

    # Start the concurrent results processor task
    results_task = asyncio.create_task(process_results(results_queue, args.total_duration, status_tracker))

    # The main loop for firing requests
    async with aiohttp.ClientSession() as session:
        start_time = time.time()
        while time.time() - start_time < args.total_duration:
            target_rps = shape.tick()
            if target_rps is None:
                break

            status_tracker['target_rps'] = target_rps

            if target_rps > 0:
                # This is the core of the open-loop model.
                # We calculate the delay needed to achieve the target RPS and then sleep for that long.
                delay = 1.0 / target_rps
                # Fire off the request and DO NOT wait for it to complete.
                asyncio.create_task(send_request(session, url, args.num_users, results_queue))
                await asyncio.sleep(delay)
            else:
                # If target RPS is 0, just wait for a second.
                await asyncio.sleep(1.0)

    print("\nLoad generation finished. Waiting for final results to be processed...")
    await results_task


if __name__ == "__main__":
    parser = argparse.ArgumentParser('True Open-Loop Load Generator')
    parser.add_argument("--num-users", type=int, default=962, help="Total number of users in the social graph (for generating random data)")
    parser.add_argument("--total-duration", type=int, default=600, help="Total experiment duration in seconds")
    parser.add_argument("--cycle-duration", type=int, default=600, help="Duration of one diurnal cycle in seconds")
    parser.add_argument("--min-rps", type=int, default=20, help="Minimum requests per second in a cycle")
    parser.add_argument("--max-rps", type=int, default=600, help="Maximum requests per second in a cycle")
    parser.add_argument('--ip', help='IP address of the target server.', default='127.0.0.1')
    parser.add_argument('--port', help='IP port of the target server.', default=8080)
    args = parser.parse_args()

    random.seed(1)  # Deterministic random data generation
    asyncio.run(main(args))